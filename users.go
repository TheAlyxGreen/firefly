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

// User represents a BlueSky user profile with basic information.
// This is the lightweight version used in most contexts like post authors or notification senders.
type User struct {
	//Associated   *ActorDefs_ProfileAssociated       `json:"associated,omitempty" cborgen:"associated,omitempty"`
	Avatar      string    `json:"avatar,omitempty" cborgen:"avatar,omitempty"`
	CreatedAt   time.Time `json:"createdAt,omitempty" cborgen:"createdAt,omitempty"`
	Description string    `json:"description,omitempty" cborgen:"description,omitempty"`
	Did         string    `json:"did" cborgen:"did"`
	DisplayName string    `json:"displayName,omitempty" cborgen:"displayName,omitempty"`
	Handle      string    `json:"handle" cborgen:"handle"`
	IndexedAt   time.Time `json:"indexedAt,omitempty" cborgen:"indexedAt,omitempty"`
	//Labels       []*comatprototypes.LabelDefs_Label `json:"labels,omitempty" cborgen:"labels,omitempty"`
	//Status       *ActorDefs_StatusView              `json:"status,omitempty" cborgen:"status,omitempty"`
	//Verification *ActorDefs_VerificationState       `json:"verification,omitempty" cborgen:"verification,omitempty"`
	//Viewer       *ActorDefs_ViewerState             `json:"viewer,omitempty" cborgen:"viewer,omitempty"`
	Raw         *bsky.ActorDefs_ProfileView
	RawDetailed *bsky.ActorDefs_ProfileViewDetailed
}

func (u *User) String() string {
	return fmt.Sprintf("User{ID: %s, Handle: %s}", u.Did, u.Handle)
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
		Avatar:      safeStringToValue(oldUser.Avatar),
		CreatedAt:   CreatedAt,
		Description: safeStringToValue(oldUser.Description),
		Did:         oldUser.Did,
		DisplayName: safeStringToValue(oldUser.DisplayName),
		Handle:      oldUser.Handle,
		IndexedAt:   IndexedAt,
		Raw:         oldUser,
		RawDetailed: nil,
	}
	return newUser, nil
}

// UserDetailed represents a BlueSky user profile with extended information including follower counts.
// This is typically used for the authenticated user's own profile (available as Firefly.Self after login)
// or when fetching complete profile information for other users.
type UserDetailed struct {
	//Associated           *ActorDefs_ProfileAssociated       `json:"associated,omitempty" cborgen:"associated,omitempty"`
	Avatar         string    `json:"avatar,omitempty" cborgen:"avatar,omitempty"`
	Banner         string    `json:"banner,omitempty" cborgen:"banner,omitempty"`
	CreatedAt      time.Time `json:"createdAt,omitempty" cborgen:"createdAt,omitempty"`
	Description    string    `json:"description,omitempty" cborgen:"description,omitempty"`
	Did            string    `json:"did" cborgen:"did"`
	DisplayName    string    `json:"displayName,omitempty" cborgen:"displayName,omitempty"`
	FollowersCount int       `json:"followersCount,omitempty" cborgen:"followersCount,omitempty"`
	FollowsCount   int       `json:"followsCount,omitempty" cborgen:"followsCount,omitempty"`
	Handle         string    `json:"handle" cborgen:"handle"`
	IndexedAt      time.Time `json:"indexedAt,omitempty" cborgen:"indexedAt,omitempty"`
	//JoinedViaStarterPack *GraphDefs_StarterPackViewBasic    `json:"joinedViaStarterPack,omitempty" cborgen:"joinedViaStarterPack,omitempty"`
	//Labels               []*comatprototypes.LabelDefs_Label `json:"labels,omitempty" cborgen:"labels,omitempty"`
	PinnedPost *PostRef `json:"pinnedPost,omitempty" cborgen:"pinnedPost,omitempty"`
	PostsCount int      `json:"postsCount,omitempty" cborgen:"postsCount,omitempty"`
	//Status               *ActorDefs_StatusView              `json:"status,omitempty" cborgen:"status,omitempty"`
	//Verification         *ActorDefs_VerificationState       `json:"verification,omitempty" cborgen:"verification,omitempty"`
	//Viewer               *ActorDefs_ViewerState             `json:"viewer,omitempty" cborgen:"viewer,omitempty"`
	Raw *bsky.ActorDefs_ProfileViewDetailed
}

func (u *UserDetailed) String() string {
	return fmt.Sprintf("UserDetailed{ID: %s, Handle: %s}", u.Did, u.Handle)
}

// OldToNewDetailedUser converts old bsky detailed user structs into new UserDetailed structs
func OldToNewDetailedUser(oldUser *bsky.ActorDefs_ProfileViewDetailed) (*UserDetailed, error) {
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
	newUser := &UserDetailed{
		Avatar:         safeStringToValue(oldUser.Avatar),
		Banner:         safeStringToValue(oldUser.Banner),
		CreatedAt:      CreatedAt,
		Description:    safeStringToValue(oldUser.Description),
		Did:            oldUser.Did,
		DisplayName:    safeStringToValue(oldUser.DisplayName),
		FollowersCount: safeI64ToInt(oldUser.FollowersCount),
		FollowsCount:   safeI64ToInt(oldUser.FollowsCount),
		Handle:         oldUser.Handle,
		IndexedAt:      IndexedAt,
		PinnedPost:     OldToNewRefPointer(oldUser.PinnedPost),
		PostsCount:     safeI64ToInt(oldUser.PostsCount),
		Raw:            oldUser,
	}
	return newUser, nil
}

// StripDetails removes all the extra info and converts a UserDetailed type into a User type
func (u *UserDetailed) StripDetails() User {
	return User{
		Avatar:      u.Avatar,
		CreatedAt:   u.CreatedAt,
		Description: u.Description,
		Did:         u.Did,
		DisplayName: u.DisplayName,
		Handle:      u.Handle,
		IndexedAt:   u.IndexedAt,
		Raw:         nil,
		RawDetailed: u.Raw,
	}
}

// GetProfile retrieves detailed profile information for a specific user.
// The actor parameter can be either a handle (e.g., "alice.bsky.social") or a DID.
//
// Example:
//   profile, err := client.GetProfile("alice.bsky.social")
//   if err != nil {
//       log.Fatal(err)
//   }
//   fmt.Printf("%s has %d followers\n", profile.DisplayName, profile.FollowersCount)
func (f *Firefly) GetProfile(actor string) (*UserDetailed, error) {
	profile, err := bsky.ActorGetProfile(f.ctx, f.client, actor)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedFetch, err)
	}
	
	return OldToNewDetailedUser(profile)
}

// GetProfiles retrieves detailed profile information for multiple users at once.
// The actors parameter should be a slice of handles or DIDs.
//
// Example:
//   profiles, err := client.GetProfiles([]string{"alice.bsky.social", "bob.bsky.social"})
//   if err != nil {
//       log.Fatal(err)
//   }
//   for _, profile := range profiles {
//       fmt.Printf("%s: %s\n", profile.Handle, profile.DisplayName)
//   }
func (f *Firefly) GetProfiles(actors []string) ([]*UserDetailed, error) {
	result, err := bsky.ActorGetProfiles(f.ctx, f.client, actors)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedFetch, err)
	}
	
	profiles := make([]*UserDetailed, len(result.Profiles))
	for i, profile := range result.Profiles {
		newProfile, err := OldToNewDetailedUser(profile)
		if err != nil {
			return nil, err
		}
		profiles[i] = newProfile
	}
	
	return profiles, nil
}

// userSearchConfig holds configuration for user search operations
type userSearchConfig struct {
	cursor string
	limit  int
}

// UserSearchOption configures user search parameters
type UserSearchOption func(*userSearchConfig)

// UserSearchWithCursor sets the cursor for paginated user search results
func UserSearchWithCursor(cursor string) UserSearchOption {
	return func(c *userSearchConfig) { c.cursor = cursor }
}

// UserSearchLimit sets the maximum number of users to return (1-100, default 25)
func UserSearchLimit(limit int) UserSearchOption {
	return func(c *userSearchConfig) { c.limit = limit }
}

// SearchUsers searches for users by query string using BlueSky's search functionality.
// The query parameter supports Lucene query syntax for advanced searches.
//
// Example:
//   users, err := client.SearchUsers("golang developer")
//   if err != nil {
//       log.Fatal(err)
//   }
//   for _, user := range users {
//       fmt.Printf("%s (@%s): %s\n", user.DisplayName, user.Handle, user.Description)
//   }
func (f *Firefly) SearchUsers(query string, opts ...UserSearchOption) ([]*User, error) {
	config := &userSearchConfig{
		limit: 25, // sensible default
	}
	
	for _, opt := range opts {
		opt(config)
	}
	
	result, err := bsky.ActorSearchActors(f.ctx, f.client, config.cursor, int64(config.limit), query, "")
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

// GetSuggestedUsers retrieves user suggestions from BlueSky's recommendation system.
// These are users that BlueSky thinks you might want to follow.
//
// Example:
//   suggestions, err := client.GetSuggestedUsers(UserSearchLimit(10))
//   if err != nil {
//       log.Fatal(err)
//   }
//   for _, user := range suggestions {
//       fmt.Printf("Suggested: %s (@%s)\n", user.DisplayName, user.Handle)
//   }
func (f *Firefly) GetSuggestedUsers(opts ...UserSearchOption) ([]*User, error) {
	config := &userSearchConfig{
		limit: 25, // sensible default
	}
	
	for _, opt := range opts {
		opt(config)
	}
	
	result, err := bsky.ActorGetSuggestions(f.ctx, f.client, config.cursor, int64(config.limit))
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
