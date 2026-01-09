package firefly

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/api/bsky"
)

var (
	ErrNilNotif     = errors.New("nil notification")
	ErrInvalidNotif = errors.New("invalid notification")
)

// NotificationReason identifies the type of activity that generated a notification.
type NotificationReason int

const (
	UnknownReason NotificationReason = iota
	NewLike
	NewRepost
	NewFollow
	NewMention
	NewReply
	NewQuote
	StarterPackJoined
	AccountVerified
	AccountUnverified
	NewLikeViaRepost
	NewRepostViaRepost
	NewSubscribedPost
	NewContactMatch
)

func (r NotificationReason) String() string {
	switch r {
	case NewLike:
		return "New Like"
	case NewRepost:
		return "New Repost"
	case NewFollow:
		return "New Follow"
	case NewMention:
		return "New Mention"
	case NewReply:
		return "New Reply"
	case NewQuote:
		return "New Quote"
	case StarterPackJoined:
		return "Starter Pack Joined"
	case AccountVerified:
		return "Account Verified"
	case AccountUnverified:
		return "Account Unverified"
	case NewLikeViaRepost:
		return "New Like Via Repost"
	case NewRepostViaRepost:
		return "New Repost Via Repost"
	case NewSubscribedPost:
		return "New Subscribed Post"
	case NewContactMatch:
		return "New Contact Match"
	default:
		return "Unknown"
	}
}

// Notification represents a BlueSky notification with information about who performed what action.
// It includes the notification reason, the user who triggered it, and any associated post.
type Notification struct {
	IndexedAt  time.Time          `json:"indexedAt" cborgen:"indexedAt"`
	IsRead     bool               `json:"isRead" cborgen:"isRead"`
	Reason     NotificationReason `json:"reason" cborgen:"reason"`
	LinkedUser *User              `json:"linkedUser" cborgen:"linkedUser"` // nil if no user linked to notif
	LinkedPost *FeedPost          `json:"linkedPost" cborgen:"linkedPost"` // nil if no post linked to notif
	Raw        *bsky.NotificationListNotifications_Notification
}

// OldToNewNotification converts bsky notifications to Firefly notifications
func (f *Firefly) OldToNewNotification(oldNotif *bsky.NotificationListNotifications_Notification) (*Notification, error) {
	if oldNotif == nil {
		return nil, ErrNilNotif
	}
	indexTime, err := time.Parse(time.RFC3339, oldNotif.IndexedAt)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidNotif, err)
	}
	newNotif := &Notification{
		LinkedUser: nil,
		IndexedAt:  indexTime,
		IsRead:     oldNotif.IsRead,
		//Labels: oldNotif.Labels,
		Reason:     UnknownReason,
		LinkedPost: nil,
		Raw:        oldNotif,
	}
	newAuthor, err := OldToNewUser(oldNotif.Author)
	if err == nil {
		newNotif.LinkedUser = newAuthor
	}

	// reasons grabbed from here: https://atproto.blue/en/latest/atproto/atproto_client.models.app.bsky.notification.list_notifications.html
	// Jan-09-25 (timestamp because they keep fucking changing it)
	switch oldNotif.Reason {
	case "like":
		newNotif.Reason = NewLike
		break
	case "mention":
		newNotif.Reason = NewMention
		break
	case "follow":
		newNotif.Reason = NewFollow
		break
	case "repost":
		newNotif.Reason = NewRepost
		break
	case "reply":
		newNotif.Reason = NewReply
		break
	case "quote":
		newNotif.Reason = NewQuote
		break
	case "starterpack-joined":
		newNotif.Reason = StarterPackJoined
		break
	case "verified":
		newNotif.Reason = AccountVerified
		break
	case "unverified":
		newNotif.Reason = AccountUnverified
		break
	case "like-via-repost":
		newNotif.Reason = NewLikeViaRepost
		break
	case "repost-via-repost":
		newNotif.Reason = NewRepostViaRepost
		break
	case "subscribed-post":
		newNotif.Reason = NewSubscribedPost
		break
	case "contact-match":
		newNotif.Reason = NewContactMatch
		break
	}
	if newNotif.Reason == NewLike ||
		newNotif.Reason == NewMention ||
		newNotif.Reason == NewRepost ||
		newNotif.Reason == NewReply ||
		newNotif.Reason == NewQuote {
		newPost, err := f.OldToNewPost(oldNotif.Record.Val.(*bsky.FeedPost), oldNotif.Uri)
		if err == nil {
			if newNotif.Reason != NewLike {
				newPost.Author = newNotif.LinkedUser
			}
			newNotif.LinkedPost = newPost
			newNotif.LinkedPost.Uri = oldNotif.Uri
			newNotif.LinkedPost.Cid = oldNotif.Cid
		}
	}

	return newNotif, nil
}

func (notif Notification) String() string {
	if notif.LinkedPost != nil {
		return fmt.Sprintf("Notification{User: %s, Reason: %s, Post: %s}",
			notif.LinkedUser.Handle, notif.Reason, notif.LinkedPost.Uri)
	}
	return fmt.Sprintf("Notification{User: %s, Reason: %s}", notif.LinkedUser.Handle, notif.Reason)
}

// GetNotifications fetches notifications from BlueSky with optional filtering.
//
// Parameters:
//   - fromBefore: Only return notifications created before this time
//   - count: Maximum number of notifications to return (1-100)
//   - priority: If true, only return notifications marked as priority by the server
//   - reasons: Filter by notification types (e.g., ["like", "follow"]). Pass nil for all types.
func (f *Firefly) GetNotifications(ctx context.Context, fromBefore time.Time, count int, priority bool, reasons []string) ([]*Notification, error) {
	notifications, err := bsky.NotificationListNotifications(ctx, f.client, fromBefore.Format(time.RFC3339), int64(count), priority, reasons, "")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedFetch, err)
	}
	var newNotifications []*Notification
	for _, notif := range notifications.Notifications {
		newNotif, err := f.OldToNewNotification(notif)
		if err != nil {
			return nil, err
		}
		if newNotif.Reason == NewLike {
			strippedUser := f.Self
			if newNotif.LinkedPost != nil {
				newNotif.LinkedPost.Author = strippedUser
			}
		}
		newNotifications = append(newNotifications, newNotif)
	}
	return newNotifications, nil
}

// GetLatestNotifications is a convenience method that returns the most recent notifications.
// This is equivalent to calling GetNotifications with time.Now() and no filters.
func (f *Firefly) GetLatestNotifications(ctx context.Context, count int) ([]*Notification, error) {
	notifications, err := f.GetNotifications(ctx, time.Now(), count, false, nil)
	if err != nil {
		return nil, err
	}
	return notifications, nil
}
