package firefly

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bluesky-social/indigo/api/atproto"
	comatprototypes "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/util"
)

var (
	ErrPostTooLong    = errors.New("post exceeds 300 characters")
	ErrInvalidMention = errors.New("invalid mention format")
	ErrInvalidLink    = errors.New("invalid link format")
)

// FragmentType identifies the type of content in a post fragment
type FragmentType int

const (
	FragmentText FragmentType = iota
	FragmentMention
	FragmentLink
	FragmentHashtag
)

func (ft FragmentType) String() string {
	switch ft {
	case FragmentText:
		return "Text"
	case FragmentMention:
		return "Mention"
	case FragmentLink:
		return "Link"
	case FragmentHashtag:
		return "Hashtag"
	default:
		return "Unknown"
	}
}

// PostFragment represents a composable piece of a post
type PostFragment struct {
	Text string       `json:"text"`
	Type FragmentType `json:"type"`

	// Type-specific data (only one will be populated based on Type)
	UserDID *string `json:"userDid,omitempty"` // For mentions
	URL     *string `json:"url,omitempty"`     // For links
	Tag     *string `json:"tag,omitempty"`     // For hashtags (without #)
}

// DraftPost represents a post being composed of fragments
type DraftPost struct {
	Fragments []PostFragment `json:"fragments"`

	// Optional post metadata
	Languages []string   `json:"languages,omitempty"` // Max 3 language codes
	Labels    []string   `json:"labels,omitempty"`    // Content warning labels
	ReplyInfo *ReplyInfo `json:"replyInfo,omitempty"` // Reply thread information
}

// NewText creates a plain text fragment
func NewText(text string) PostFragment {
	return PostFragment{
		Text: text,
		Type: FragmentText,
	}
}

// NewPostMention creates a mention fragment
// userID can be either a handle (alice.bsky.social) or DID (did:plc:...)
func NewPostMention(displayText, userID string) PostFragment {
	return PostFragment{
		Text:    displayText,
		Type:    FragmentMention,
		UserDID: &userID, // We'll resolve handles to DIDs later
	}
}

// NewLink creates a link fragment
func NewLink(displayText, url string) PostFragment {
	return PostFragment{
		Text: displayText,
		Type: FragmentLink,
		URL:  &url,
	}
}

// NewHashtag creates a hashtag fragment
func NewHashtag(tag string) PostFragment {
	// Remove # prefix if present
	tag = strings.TrimPrefix(tag, "#")
	return PostFragment{
		Text: "#" + tag,
		Type: FragmentHashtag,
		Tag:  &tag,
	}
}

// NewDraftPost creates a new empty draft post
func NewDraftPost() *DraftPost {
	return &DraftPost{
		Fragments: make([]PostFragment, 0),
	}
}

// AddFragment appends a fragment to the draft post (chainable)
func (d *DraftPost) AddFragment(fragment PostFragment) *DraftPost {
	d.Fragments = append(d.Fragments, fragment)
	return d
}

// AddText is a convenience method to add plain text
func (d *DraftPost) AddText(text string) *DraftPost {
	return d.AddFragment(NewText(text))
}

// AddMention is a convenience method to add a mention
func (d *DraftPost) AddMention(displayText, userID string) *DraftPost {
	return d.AddFragment(NewPostMention(displayText, userID))
}

// AddLink is a convenience method to add a link
func (d *DraftPost) AddLink(displayText, url string) *DraftPost {
	return d.AddFragment(NewLink(displayText, url))
}

// AddHashtag is a convenience method to add a hashtag
func (d *DraftPost) AddHashtag(tag string) *DraftPost {
	return d.AddFragment(NewHashtag(tag))
}

// SetLanguages sets the language codes for the post (max 3)
func (d *DraftPost) SetLanguages(languages ...string) *DraftPost {
	if len(languages) > 3 {
		languages = languages[:3] // Truncate to BlueSky's limit
	}
	d.Languages = languages
	return d
}

// SetLabels sets content warning labels for the post
// Common values: "porn", "sexual", "nudity", "graphic-media"
func (d *DraftPost) SetLabels(labels ...string) *DraftPost {
	d.Labels = labels
	return d
}

// AddLabel adds a content warning label to the post
func (d *DraftPost) AddLabel(label string) *DraftPost {
	d.Labels = append(d.Labels, label)
	return d
}

// SetReplyInfo sets up a reply to another post
// For simple replies (replying directly to original post), pass the same PostRef for both parent and root
// For thread replies, pass the immediate parent and the thread root separately
func (d *DraftPost) SetReplyInfo(parent, root *PostRef) *DraftPost {
	d.ReplyInfo = &ReplyInfo{
		ReplyTarget: parent,
		ReplyRoot:   root,
	}
	return d
}

// PostReply creates a reply to an existing post, automatically handling thread structure
// If originalPost is a root post, the new reply becomes a direct reply
// If originalPost is already a reply in a thread, the new reply maintains the thread root
func (f *Firefly) PostReply(ctx context.Context, originalPost *FeedPost, newPost *DraftPost) (*PostRef, error) {
	// Determine parent and root based on the original post's reply structure
	parent := &PostRef{
		Uri: originalPost.Uri,
		Cid: originalPost.Cid,
	}

	var root *PostRef
	if originalPost.ReplyInfo != nil && originalPost.ReplyInfo.ReplyRoot != nil {
		// Original post is part of a thread, use its root as our root
		root = originalPost.ReplyInfo.ReplyRoot
	} else {
		// Original post is a root post, it becomes the root for our reply
		root = parent
	}

	// Set up the reply structure
	newPost.SetReplyInfo(parent, root)

	// Create the post normally - the reply structure is now set
	return f.PublishDraftPost(ctx, newPost)
}

// GetText returns the complete text content of the draft post
func (d *DraftPost) GetText() string {
	var text strings.Builder
	for _, fragment := range d.Fragments {
		text.WriteString(fragment.Text)
	}
	return text.String()
}

// GetCharacterCount returns the number of graphemes (user-visible characters)
func (d *DraftPost) GetCharacterCount() int {
	return utf8.RuneCountInString(d.GetText())
}

// IsValid checks if the draft post meets BlueSky's requirements
func (d *DraftPost) IsValid() error {
	text := d.GetText()

	// Check character limit (300 graphemes)
	if utf8.RuneCountInString(text) > 300 {
		return ErrPostTooLong
	}

	// Check byte limit (3000 bytes)
	if len(text) > 3000 {
		return ErrPostTooLong
	}

	return nil
}

// DraftToBskyPost converts the draft post to a BlueSky FeedPost with automatic facet generation.
//
// Note: This method performs network requests to resolve user handles to DIDs if mentions
// are present in the draft. Ensure the provided context is valid.
func (f *Firefly) DraftToBskyPost(ctx context.Context, draft *DraftPost) (*bsky.FeedPost, error) {
	// Validate the post first
	if err := draft.IsValid(); err != nil {
		return nil, err
	}

	// Build the complete text and track positions
	var textBuilder strings.Builder
	var facets []*bsky.RichtextFacet
	currentBytePos := 0

	for _, fragment := range draft.Fragments {
		fragmentBytes := []byte(fragment.Text)
		fragmentByteLength := len(fragmentBytes)

		// Add text to the complete post
		textBuilder.Write(fragmentBytes)

		// Create facet for non-text fragments
		switch fragment.Type {
		case FragmentMention:
			if fragment.UserDID == nil {
				return nil, fmt.Errorf("%w: missing user ID", ErrInvalidMention)
			}

			// Resolve handle to DID if needed
			userDID := *fragment.UserDID
			if !strings.HasPrefix(userDID, "did:") {
				// This is a handle, resolve it to DID
				resolvedDID, err := f.ResolveHandleToDID(ctx, userDID)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve handle %s: %w", userDID, err)
				}
				userDID = resolvedDID
			}

			facet := &bsky.RichtextFacet{
				Index: &bsky.RichtextFacet_ByteSlice{
					ByteStart: int64(currentBytePos),
					ByteEnd:   int64(currentBytePos + fragmentByteLength),
				},
				Features: []*bsky.RichtextFacet_Features_Elem{
					&bsky.RichtextFacet_Features_Elem{
						RichtextFacet_Mention: &bsky.RichtextFacet_Mention{
							Did: userDID,
						},
					},
				},
			}
			facets = append(facets, facet)

		case FragmentLink:
			if fragment.URL == nil {
				return nil, fmt.Errorf("%w: missing URL", ErrInvalidLink)
			}

			facet := &bsky.RichtextFacet{
				Index: &bsky.RichtextFacet_ByteSlice{
					ByteStart: int64(currentBytePos),
					ByteEnd:   int64(currentBytePos + fragmentByteLength),
				},
				Features: []*bsky.RichtextFacet_Features_Elem{
					&bsky.RichtextFacet_Features_Elem{
						RichtextFacet_Link: &bsky.RichtextFacet_Link{
							Uri: *fragment.URL,
						},
					},
				},
			}
			facets = append(facets, facet)

		case FragmentHashtag:
			if fragment.Tag == nil {
				return nil, fmt.Errorf("hashtag fragment missing tag")
			}

			facet := &bsky.RichtextFacet{
				Index: &bsky.RichtextFacet_ByteSlice{
					ByteStart: int64(currentBytePos),
					ByteEnd:   int64(currentBytePos + fragmentByteLength),
				},
				Features: []*bsky.RichtextFacet_Features_Elem{
					&bsky.RichtextFacet_Features_Elem{
						RichtextFacet_Tag: &bsky.RichtextFacet_Tag{
							Tag: *fragment.Tag,
						},
					},
				},
			}
			facets = append(facets, facet)
		}

		currentBytePos += fragmentByteLength
	}

	// Create the BlueSky post
	post := &bsky.FeedPost{
		Text:      textBuilder.String(),
		CreatedAt: time.Now().Format(util.ISO8601),
	}

	// Add facets if any exist
	if len(facets) > 0 {
		post.Facets = facets
	}

	// Add languages if specified
	if len(draft.Languages) > 0 {
		post.Langs = draft.Languages
	}

	// Add labels (content warnings) if specified
	if len(draft.Labels) > 0 {
		selfLabels := make([]*comatprototypes.LabelDefs_SelfLabel, len(draft.Labels))
		for i, label := range draft.Labels {
			selfLabels[i] = &comatprototypes.LabelDefs_SelfLabel{
				Val: label,
			}
		}
		post.Labels = &bsky.FeedPost_Labels{
			LabelDefs_SelfLabels: &comatprototypes.LabelDefs_SelfLabels{
				LexiconTypeID: "com.atproto.label.defs#selfLabels",
				Values:        selfLabels,
			},
		}
	}

	// Add reply information if this is a reply
	if draft.ReplyInfo != nil {
		post.Reply = &bsky.FeedPost_ReplyRef{
			Parent: &atproto.RepoStrongRef{
				Uri: draft.ReplyInfo.ReplyTarget.Uri,
				Cid: draft.ReplyInfo.ReplyTarget.Cid,
			},
			Root: &atproto.RepoStrongRef{
				Uri: draft.ReplyInfo.ReplyRoot.Uri,
				Cid: draft.ReplyInfo.ReplyRoot.Cid,
			},
		}
	}

	return post, nil
}

// PublishDraftPost publishes a draft post to BlueSky.
//
// Note: This method performs network requests to resolve user handles to DIDs if mentions
// are present in the draft (via DraftToBskyPost).
func (f *Firefly) PublishDraftPost(ctx context.Context, draft *DraftPost) (*PostRef, error) {
	// Convert to BlueSky format with automatic facet generation
	bskyPost, err := f.DraftToBskyPost(ctx, draft)
	if err != nil {
		return nil, fmt.Errorf("failed to convert draft post: %w", err)
	}

	// Create the post using BlueSky's API
	resp, err := atproto.RepoCreateRecord(ctx, f.client, &atproto.RepoCreateRecord_Input{
		Collection: "app.bsky.feed.post",
		Repo:       f.Self.Did, // Use authenticated user's DID
		Record: &lexutil.LexiconTypeDecoder{
			Val: bskyPost,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create post: %w", err)
	}

	// Return a PostRef for the created post
	return &PostRef{
		Uri: resp.Uri,
		Cid: resp.Cid,
	}, nil
}
