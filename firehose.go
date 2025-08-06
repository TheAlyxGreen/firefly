package firefly

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/bluesky-social/jetstream/pkg/models"
	"github.com/gorilla/websocket"
)

var (
	ErrFirehoseFailed     = errors.New("firehose connection failed")
	ErrFirehoseDisconnect = errors.New("firehose disconnected")
	ErrInvalidEvent       = errors.New("invalid firehose event")
)

// FirehoseEventType identifies the type of activity in a firehose event
type FirehoseEventType int

const (
	EventTypeUnknown FirehoseEventType = iota
	EventTypePost
	EventTypeLike
	EventTypeFollow
	EventTypeProfile
	EventTypeDelete
	EventTypeRepost
)

func (et FirehoseEventType) String() string {
	switch et {
	case EventTypePost:
		return "Post"
	case EventTypeLike:
		return "Like"
	case EventTypeFollow:
		return "Follow"
	case EventTypeProfile:
		return "Profile"
	case EventTypeDelete:
		return "Delete"
	case EventTypeRepost:
		return "Repost"
	default:
		return "Unknown"
	}
}

// FirehoseEvent represents a simplified firehose event using existing Firefly types
type FirehoseEvent struct {
	Type      FirehoseEventType `json:"type"`
	Sequence  int64             `json:"sequence"`
	Repo      string            `json:"repo"` // Author DID
	Timestamp time.Time         `json:"timestamp"`

	// Event-specific data (only one will be populated)
	Post        *FeedPost       `json:"post,omitempty"`
	User        *User           `json:"user,omitempty"`        // For profile updates, follows
	DeleteEvent *FirehoseDelete `json:"deleteEvent,omitempty"` // For deletions
	LikeEvent   *FirehoseLike   `json:"likeEvent,omitempty"`   // For likes
	RepostEvent *FirehoseRepost `json:"repostEvent,omitempty"` // For reposts

	// Raw Jetstream data preservation
	RawCommit *models.Event
}

// FirehoseDelete represents a deletion event from the firehose
type FirehoseDelete struct {
	Collection string `json:"collection"` // e.g. "app.bsky.feed.post"
	RecordKey  string `json:"recordKey"`  // Record that was deleted
	URI        string `json:"uri"`        // Full AT URI of deleted record
}

// FirehoseLike represents a like event from the firehose
type FirehoseLike struct {
	Subject *PostRef `json:"subject"` // Post being liked
	URI     string   `json:"uri"`     // URI of the like record
}

// FirehoseRepost represents a repost event from the firehose
type FirehoseRepost struct {
	Subject *PostRef `json:"subject"` // Post being reposted
	URI     string   `json:"uri"`     // URI of the repost record
}

// FirehoseOptions configures firehose filtering and behavior
type FirehoseOptions struct {
	Collections  []string `json:"collections,omitempty"`  // Filter by collection types (max 100)
	Authors      []string `json:"authors,omitempty"`      // Filter by author DIDs/handles (max 10,000)
	Cursor       *int64   `json:"cursor,omitempty"`       // Resume from Unix microsecond timestamp
	BufferSize   int      `json:"bufferSize,omitempty"`   // Channel buffer size (default 1000)
	Compression  bool     `json:"compression,omitempty"`  // Enable zstd compression
	RequireHello bool     `json:"requireHello,omitempty"` // Pause until initial config
}

// StreamEvents opens a firehose connection with advanced filtering options
// Uses options struct for complex configuration following Firefly's API patterns
func (f *Firefly) StreamEvents(ctx context.Context, options *FirehoseOptions) (chan *FirehoseEvent, error) {
	if options == nil {
		options = &FirehoseOptions{}
	}

	// Set defaults
	if options.BufferSize <= 0 {
		options.BufferSize = 1000
	}

	// If no collections are specified, default to the main content types
	// This prevents getting flooded with account/identity events
	if len(options.Collections) == 0 {
		options.Collections = []string{
			"app.bsky.feed.post",
			"app.bsky.feed.like",
			"app.bsky.feed.repost",
			"app.bsky.graph.follow",
		}
	}

	// Create buffered channel for events
	events := make(chan *FirehoseEvent, options.BufferSize)

	// Start background goroutine to manage connection
	go func() {
		defer close(events)
		f.maintainFirehoseConnection(ctx, options, events)
	}()

	return events, nil
}

// maintainFirehoseConnection handles connection lifecycle with reconnection logic
func (f *Firefly) maintainFirehoseConnection(ctx context.Context, options *FirehoseOptions, events chan<- *FirehoseEvent) {
	backoff := time.Second
	maxBackoff := time.Minute * 2

	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := f.connectFirehose(ctx, options, events)
			if err != nil {
				// Send error to ErrorChan following Firefly's error handling pattern
				select {
				case f.ErrorChan <- fmt.Errorf("%w: %w", ErrFirehoseFailed, err):
				default:
					// ErrorChan is full, error will be lost but we won't block
				}

				// Exponential backoff
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
					if backoff < maxBackoff {
						backoff *= 2
					}
				}
				continue
			}
			// Reset backoff on successful connection
			backoff = time.Second
		}
	}
}

// connectFirehose establishes a single WebSocket connection to the Jetstream firehose
func (f *Firefly) connectFirehose(ctx context.Context, options *FirehoseOptions, events chan<- *FirehoseEvent) error {
	// Build Jetstream WebSocket URL
	url := f.buildJetstreamURL(options)

	// Setup WebSocket dialer
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	// Connect to WebSocket
	conn, _, err := dialer.Dial(url, http.Header{})
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}
	defer conn.Close()

	// Set read deadline for keep-alive
	conn.SetReadDeadline(time.Now().Add(time.Minute * 5))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(time.Minute * 5))
		return nil
	})

	// Start ping routine for keep-alive
	pingTicker := time.NewTicker(time.Minute)
	defer pingTicker.Stop()

	go func() {
		for {
			select {
			case <-pingTicker.C:
				conn.WriteMessage(websocket.PingMessage, []byte{})
			case <-ctx.Done():
				return
			}
		}
	}()

	// Read messages from WebSocket
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			_, message, err := conn.ReadMessage()
			if err != nil {
				return fmt.Errorf("%w: %w", ErrFirehoseDisconnect, err)
			}

			// Process the message
			event, err := f.processFirehoseMessage(message)
			if err != nil {
				// Log error but continue processing
				select {
				case f.ErrorChan <- fmt.Errorf("%w: %w", ErrInvalidEvent, err):
				default:
				}
				continue
			}

			if event != nil {
				// Send event to channel (non-blocking)
				select {
				case events <- event:
				case <-ctx.Done():
					return nil
				default:
					// Channel is full, drop the event
					// Could log this or increment a metric
				}
			}
		}
	}
}

// buildJetstreamURL constructs the Jetstream WebSocket URL with query parameters
func (f *Firefly) buildJetstreamURL(options *FirehoseOptions) string {
	possibleUrls := []string{
		"wss://jetstream1.us-east.bsky.network/subscribe",
		"wss://jetstream2.us-east.bsky.network/subscribe",
		"wss://jetstream1.us-west.bsky.network/subscribe",
		"wss://jetstream2.us-west.bsky.network/subscribe",
	}
	// Use random endpoint
	baseURL := possibleUrls[rand.Intn(4)]

	var params []string

	if len(options.Collections) > 0 {
		// Limit to max 100 collections as per Jetstream spec
		collections := options.Collections
		if len(collections) > 100 {
			collections = collections[:100]
		}
		collectionsString := strings.Join(collections, "&wantedCollections=")
		collectionsString = strings.TrimSuffix(collectionsString, "&wantedCollections=")
		params = append(params, "wantedCollections="+collectionsString)
	}

	if len(options.Authors) > 0 {
		// Limit to max 10,000 DIDs as per Jetstream spec
		authors := options.Authors
		if len(authors) > 10000 {
			authors = authors[:10000]
		}
		authorsString := strings.Join(authors, "&wantedDids=")
		authorsString = strings.TrimSuffix(authorsString, "&wantedDids=")
		params = append(params, "wantedDids="+authorsString)
	}

	if options.Cursor != nil {
		params = append(params, fmt.Sprintf("cursor=%d", *options.Cursor))
	}

	if options.Compression {
		params = append(params, "compress=true")
	}

	if options.RequireHello {
		params = append(params, "requireHello=true")
	}

	if len(params) > 0 {
		baseURL += "?" + strings.Join(params, "&")
	}

	return baseURL
}

// processFirehoseMessage converts a raw Jetstream message to a FirehoseEvent
func (f *Firefly) processFirehoseMessage(message []byte) (*FirehoseEvent, error) {
	var rawCommit models.Event
	if err := json.Unmarshal(message, &rawCommit); err != nil {
		return nil, fmt.Errorf("failed to unmarshal jetstream message: %w", err)
	}

	// Convert timestamp from microseconds to time.Time
	timestamp := time.Unix(0, rawCommit.TimeUS*1000)

	// Create base event
	event := &FirehoseEvent{
		Type:      EventTypeUnknown,
		Sequence:  rawCommit.TimeUS, // Use timestamp as sequence for now
		Repo:      rawCommit.Did,
		Timestamp: timestamp,
		RawCommit: &rawCommit,
	}

	// Process based on event kind
	switch rawCommit.Kind {
	case "commit":
		return f.processCommitEvent(event, &rawCommit)
	case "identity":
		return f.processIdentityEvent(event, &rawCommit)
	case "account":
		return f.processAccountEvent(event, &rawCommit)
	default:
		// Unknown event type, return as-is
		return event, nil
	}
}
