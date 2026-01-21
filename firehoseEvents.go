package firefly

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/jetstream/pkg/models"
)

// processCommitEvent handles repository commit events (posts, likes, follows, etc.)
func (f *Firefly) processCommitEvent(event *FirehoseEvent, commit *models.Event) (*FirehoseEvent, error) {
	if commit.Commit == nil {
		return nil, fmt.Errorf("commit event missing commit data")
	}

	commitData := commit.Commit
	collection := commitData.Collection

	// Determine event type based on collection
	// Collections should be exact matches
	switch collection {
	case "app.bsky.feed.post":
		return f.processPostEvent(event, commitData)
	case "app.bsky.feed.like":
		return f.processLikeEvent(event, commitData)
	case "app.bsky.feed.repost":
		return f.processRepostEvent(event, commitData)
	case "app.bsky.graph.follow":
		return f.processFollowEvent(event, commitData)
	case "app.bsky.actor.profile":
		return f.processProfileEvent(event, commitData)
	default:
		// Unknown collection type - this might help debug what collections we're actually getting
		event.Type = EventTypeUnknown
		return event, nil
	}
}

// processPostEvent handles feed post creation, updates, and deletions
func (f *Firefly) processPostEvent(event *FirehoseEvent, commit *models.Commit) (*FirehoseEvent, error) {
	if commit.Operation == "delete" {
		// Post deletion
		event.Type = EventTypeDelete
		event.DeleteEvent = &FirehoseDelete{
			Collection: commit.Collection,
			RecordKey:  commit.RKey,
			URI:        fmt.Sprintf("at://%s/%s/%s", event.Repo, commit.Collection, commit.RKey),
		}
		return event, nil
	}

	// Post creation or update
	if commit.Record == nil {
		return nil, fmt.Errorf("post event missing record data")
	}

	// Parse the record as a BlueSky post
	var bskyPost bsky.FeedPost
	if err := json.Unmarshal(commit.Record, &bskyPost); err != nil {
		return nil, fmt.Errorf("failed to unmarshal post record: %w", err)
	}

	// Convert to Firefly FeedPost using existing conversion function
	fireflyPost, err := f.OldToNewPost(&bskyPost, event.Repo)
	if err != nil {
		return nil, fmt.Errorf("failed to convert post: %w", err)
	}

	// Set URI and CID from commit data
	fireflyPost.URI = fmt.Sprintf("at://%s/%s/%s", event.Repo, commit.Collection, commit.RKey)
	fireflyPost.CID = commit.CID

	event.Type = EventTypePost
	event.Post = fireflyPost
	return event, nil
}

// processLikeEvent handles like creation and deletion
func (f *Firefly) processLikeEvent(event *FirehoseEvent, commit *models.Commit) (*FirehoseEvent, error) {
	if commit.Operation == "delete" {
		// Like deletion
		event.Type = EventTypeDelete
		event.DeleteEvent = &FirehoseDelete{
			Collection: commit.Collection,
			RecordKey:  commit.RKey,
			URI:        fmt.Sprintf("at://%s/%s/%s", event.Repo, commit.Collection, commit.RKey),
		}
		return event, nil
	}

	// Like creation
	if commit.Record == nil {
		return nil, fmt.Errorf("like event missing record data")
	}

	// Parse the like record
	var likeRecord struct {
		Subject struct {
			URI string `json:"uri"`
			CID string `json:"cid"`
		} `json:"subject"`
		CreatedAt string `json:"createdAt"`
	}

	if err := json.Unmarshal(commit.Record, &likeRecord); err != nil {
		return nil, fmt.Errorf("failed to unmarshal like record: %w", err)
	}

	event.Type = EventTypeLike
	event.LikeEvent = &FirehoseLike{
		Subject: &PostRef{
			URI: likeRecord.Subject.URI,
			CID: likeRecord.Subject.CID,
		},
		URI: fmt.Sprintf("at://%s/%s/%s", event.Repo, commit.Collection, commit.RKey),
	}
	return event, nil
}

// processRepostEvent handles repost creation and deletion
func (f *Firefly) processRepostEvent(event *FirehoseEvent, commit *models.Commit) (*FirehoseEvent, error) {
	if commit.Operation == "delete" {
		// Repost deletion
		event.Type = EventTypeDelete
		event.DeleteEvent = &FirehoseDelete{
			Collection: commit.Collection,
			RecordKey:  commit.RKey,
			URI:        fmt.Sprintf("at://%s/%s/%s", event.Repo, commit.Collection, commit.RKey),
		}
		return event, nil
	}

	// Repost creation
	if commit.Record == nil {
		return nil, fmt.Errorf("repost event missing record data")
	}

	// Parse the repost record
	var repostRecord struct {
		Subject struct {
			URI string `json:"uri"`
			CID string `json:"cid"`
		} `json:"subject"`
		CreatedAt string `json:"createdAt"`
	}

	if err := json.Unmarshal(commit.Record, &repostRecord); err != nil {
		return nil, fmt.Errorf("failed to unmarshal repost record: %w", err)
	}

	event.Type = EventTypeRepost
	event.RepostEvent = &FirehoseRepost{
		Subject: &PostRef{
			URI: repostRecord.Subject.URI,
			CID: repostRecord.Subject.CID,
		},
		URI: fmt.Sprintf("at://%s/%s/%s", event.Repo, commit.Collection, commit.RKey),
	}
	return event, nil
}

// processFollowEvent handles follow creation and deletion
func (f *Firefly) processFollowEvent(event *FirehoseEvent, commit *models.Commit) (*FirehoseEvent, error) {
	if commit.Operation == "delete" {
		// Follow deletion (unfollow)
		event.Type = EventTypeDelete
		event.DeleteEvent = &FirehoseDelete{
			Collection: commit.Collection,
			RecordKey:  commit.RKey,
			URI:        fmt.Sprintf("at://%s/%s/%s", event.Repo, commit.Collection, commit.RKey),
		}
		return event, nil
	}

	// Follow creation
	if commit.Record == nil {
		return nil, fmt.Errorf("follow event missing record data")
	}

	// Parse the follow record
	var followRecord struct {
		Subject   string `json:"subject"` // DID being followed
		CreatedAt string `json:"createdAt"`
	}

	if err := json.Unmarshal(commit.Record, &followRecord); err != nil {
		return nil, fmt.Errorf("failed to unmarshal follow record: %w", err)
	}

	// Create a minimal User object for the follow target
	// We only have the DID, so other fields will be nil/empty
	targetUser := &User{
		Did:    followRecord.Subject,
		Handle: "", // We don't have this from the follow record
	}

	event.Type = EventTypeFollow
	event.User = targetUser
	return event, nil
}

// processProfileEvent handles profile updates
func (f *Firefly) processProfileEvent(event *FirehoseEvent, commit *models.Commit) (*FirehoseEvent, error) {
	if commit.Operation == "delete" {
		// Profile deletion
		event.Type = EventTypeDelete
		event.DeleteEvent = &FirehoseDelete{
			Collection: commit.Collection,
			RecordKey:  commit.RKey,
			URI:        fmt.Sprintf("at://%s/%s/%s", event.Repo, commit.Collection, commit.RKey),
		}
		return event, nil
	}

	// Profile creation or update
	if commit.Record == nil {
		return nil, fmt.Errorf("profile event missing record data")
	}

	// Parse the profile record
	var profileRecord bsky.ActorProfile
	if err := json.Unmarshal(commit.Record, &profileRecord); err != nil {
		return nil, fmt.Errorf("failed to unmarshal profile record: %w", err)
	}

	// Handle Avatar conversion from LexBlob to string
	var avatarStr *string
	if profileRecord.Avatar != nil {
		s := profileRecord.Avatar.Ref.String()
		avatarStr = &s
	}

	// Handle IndexedAt conversion
	indexedAtStr := event.Timestamp.Format("2006-01-02T15:04:05.000Z")

	// Convert to a basic ProfileViewBasic structure for conversion
	// Note: ProfileViewBasic doesn't have Description field
	profileViewBasic := &bsky.ActorDefs_ProfileViewBasic{
		Did:         event.Repo,
		Handle:      "", // We don't have handle in profile record
		DisplayName: profileRecord.DisplayName,
		Avatar:      avatarStr,
		CreatedAt:   &indexedAtStr,
	}

	// Convert to Firefly User using existing conversion function
	fireflyUser, err := OldToNewUserBasic(profileViewBasic)
	if err != nil {
		return nil, fmt.Errorf("failed to convert profile: %w", err)
	}

	event.Type = EventTypeProfile
	event.User = fireflyUser
	return event, nil
}

// processIdentityEvent handles identity changes (handle updates, etc.)
func (f *Firefly) processIdentityEvent(event *FirehoseEvent, commit *models.Event) (*FirehoseEvent, error) {
	if commit.Identity == nil {
		return nil, fmt.Errorf("identity event missing identity data")
	}

	identity := commit.Identity

	// Create a User object with the updated identity information
	var handle string
	if identity.Handle != nil {
		handle = *identity.Handle
	}
	user := &User{
		Did:    identity.Did,
		Handle: handle,
	}
	var identEvent FirehoseIdentity
	identEvent.DID = identity.Did
	identEvent.Handle = handle
	identEvent.Seq = identity.Seq

	timestamp, err := time.Parse(time.RFC3339, identity.Time)
	if err == nil {
		identEvent.Time = timestamp
	}

	event.IdentityEvent = &identEvent
	event.Type = EventTypeIdentity
	event.User = user
	return event, nil
}

// processAccountEvent handles account status changes (active/inactive, etc.)
func (f *Firefly) processAccountEvent(event *FirehoseEvent, commit *models.Event) (*FirehoseEvent, error) {
	if commit.Account == nil {
		return nil, fmt.Errorf("account event missing account data")
	}

	account := commit.Account

	// Create a minimal User object with account status information
	// We could extend the User type to include an Active field if needed
	user := &User{
		Did:    account.Did,
		Handle: "", // Not available in account events
	}

	var accountEvent FirehoseAccount
	accountEvent.DID = account.Did
	accountEvent.Active = account.Active
	accountEvent.Seq = account.Seq

	if account.Status != nil {
		accountEvent.Status = *account.Status
	}

	timestamp, err := time.Parse(time.RFC3339, account.Time)
	if err == nil {
		accountEvent.Time = timestamp
	}

	event.AccountEvent = &accountEvent
	event.Type = EventTypeAccount
	event.User = user
	return event, nil
}
