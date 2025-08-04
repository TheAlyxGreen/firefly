package firefly

import (
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/api/bsky"
)

var (
	ErrNilUser     = errors.New("nil user")
	ErrInvalidUser = errors.New("invalid user")
)

// User represents a BlueSky user profile that can contain either basic or detailed information.
// Optional fields use pointers for nil-safe handling. Detailed info (follower counts, etc.) may be nil for basic profiles.
type User struct {
	Avatar         *string    `json:"avatar,omitempty" cborgen:"avatar,omitempty"`
	Banner         *string    `json:"banner,omitempty" cborgen:"banner,omitempty"`
	CreatedAt      time.Time  `json:"createdAt,omitempty" cborgen:"createdAt,omitempty"`
	Description    *string    `json:"description,omitempty" cborgen:"description,omitempty"`
	Did            string     `json:"did" cborgen:"did"`
	DisplayName    *string    `json:"displayName,omitempty" cborgen:"displayName,omitempty"`
	Handle         string     `json:"handle" cborgen:"handle"`
	IndexedAt      *time.Time `json:"indexedAt,omitempty" cborgen:"indexedAt,omitempty"`
	FollowersCount *int       `json:"followersCount,omitempty" cborgen:"followersCount,omitempty"`
	FollowsCount   *int       `json:"followsCount,omitempty" cborgen:"followsCount,omitempty"`
	PinnedPost     *PostRef   `json:"pinnedPost,omitempty" cborgen:"pinnedPost,omitempty"`
	PostsCount     *int       `json:"postsCount,omitempty" cborgen:"postsCount,omitempty"`
	RawBasic       *bsky.ActorDefs_ProfileViewBasic
	Raw            *bsky.ActorDefs_ProfileView
	RawDetailed    *bsky.ActorDefs_ProfileViewDetailed
	//Associated   *ActorDefs_ProfileAssociated       `json:"associated,omitempty" cborgen:"associated,omitempty"`
	//Labels       []*comatprototypes.LabelDefs_Label `json:"labels,omitempty" cborgen:"labels,omitempty"`
	//Status       *ActorDefs_StatusView              `json:"status,omitempty" cborgen:"status,omitempty"`
	//Verification *ActorDefs_VerificationState       `json:"verification,omitempty" cborgen:"verification,omitempty"`
	//Viewer       *ActorDefs_ViewerState             `json:"viewer,omitempty" cborgen:"viewer,omitempty"`
}

func (u *User) String() string {
	return fmt.Sprintf("User{DID: %s, Handle: %s}", u.Did, u.Handle)
}

func OldToNewUserBasic(oldUser *bsky.ActorDefs_ProfileViewBasic) (*User, error) {
	if oldUser == nil {
		return nil, ErrNilUser
	}
	CreatedAt, err := time.Parse(time.RFC3339, *oldUser.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidUser, err)
	}
	return &User{
		Avatar:      oldUser.Avatar,
		CreatedAt:   CreatedAt,
		Did:         oldUser.Did,
		DisplayName: oldUser.DisplayName,
		Handle:      oldUser.Handle,
		RawBasic:    oldUser,
	}, nil
}

func OldToNewUser(oldUser *bsky.ActorDefs_ProfileView) (*User, error) {
	if oldUser == nil {
		return nil, ErrNilUser
	}
	CreatedAt, err := time.Parse(time.RFC3339, *oldUser.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidUser, err)
	}
	IndexedAt, err := time.Parse(time.RFC3339, *oldUser.IndexedAt)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidUser, err)
	}
	newUser := &User{
		Avatar:      oldUser.Avatar,
		CreatedAt:   CreatedAt,
		Description: oldUser.Description,
		Did:         oldUser.Did,
		DisplayName: oldUser.DisplayName,
		Handle:      oldUser.Handle,
		IndexedAt:   &IndexedAt,
		Raw:         oldUser,
		RawDetailed: nil,
	}
	return newUser, nil
}

// OldToNewDetailedUser converts old bsky detailed user structs into new User structs
func OldToNewDetailedUser(oldUser *bsky.ActorDefs_ProfileViewDetailed) (*User, error) {
	if oldUser == nil {
		return nil, ErrNilUser
	}
	CreatedAt, err := time.Parse(time.RFC3339, *oldUser.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidUser, err)
	}
	IndexedAt, err := time.Parse(time.RFC3339, *oldUser.IndexedAt)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidUser, err)
	}
	var followersCount int
	if oldUser.FollowersCount != nil {
		followersCount = int(*oldUser.FollowersCount)
	}
	var followsCount int
	if oldUser.FollowsCount != nil {
		followsCount = int(*oldUser.FollowsCount)
	}
	var postsCount int
	if oldUser.PostsCount != nil {
		postsCount = int(*oldUser.PostsCount)
	}

	newUser := &User{
		Avatar:         oldUser.Avatar,
		Banner:         oldUser.Banner,
		CreatedAt:      CreatedAt,
		Description:    oldUser.Description,
		Did:            oldUser.Did,
		DisplayName:    oldUser.DisplayName,
		FollowersCount: &followersCount,
		FollowsCount:   &followsCount,
		Handle:         oldUser.Handle,
		IndexedAt:      &IndexedAt,
		PinnedPost:     OldToNewRefPointer(oldUser.PinnedPost),
		PostsCount:     &postsCount,
		RawDetailed:    oldUser,
	}
	return newUser, nil
}

// GetProfile retrieves detailed profile information for a specific user.
// The actor parameter can be either a handle (e.g., "alice.bsky.social") or a DID.
//
// Example:
//
//	profile, err := client.GetProfile("alice.bsky.social")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if profile.FollowersCount != nil {
//	    fmt.Printf("%s has %d followers\n", *profile.DisplayName, *profile.FollowersCount)
//	}
func (f *Firefly) GetProfile(actor string) (*User, error) {
	profile, err := bsky.ActorGetProfile(f.ctx, f.client, actor)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedFetch, err)
	}

	return OldToNewDetailedUser(profile)
}

// SearchUsers searches for BlueSky users matching the query string.
// Returns basic user profiles (detailed fields like follower counts may be nil).
func (f *Firefly) SearchUsers(query string, cursor string, limit int) ([]*User, error) {

	result, err := bsky.ActorSearchActors(f.ctx, f.client, cursor, int64(limit), query, "")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedFetch, err)
	}

	users := make([]*User, len(result.Actors))
	for i, actor := range result.Actors {
		newUser, err := OldToNewUser(actor)
		if err != nil {
			return nil, err
		}
		users[i] = newUser
	}

	return users, nil
}

// GetSuggestedUsers returns user suggestions from BlueSky's recommendation algorithm.
// Returns basic user profiles (detailed fields like follower counts may be nil).
func (f *Firefly) GetSuggestedUsers(cursor string, limit int) ([]*User, error) {

	result, err := bsky.ActorGetSuggestions(f.ctx, f.client, cursor, int64(limit))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedFetch, err)
	}

	users := make([]*User, len(result.Actors))
	for i, actor := range result.Actors {
		newUser, err := OldToNewUser(actor)
		if err != nil {
			return nil, err
		}
		users[i] = newUser
	}

	return users, nil
}
