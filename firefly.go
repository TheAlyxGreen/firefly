package firefly

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/golang-jwt/jwt/v5"
)

const defaultBskyServer = "https://bsky.social"

var (
	ErrBadLogin           = errors.New("bad login credentials")
	ErrBadServer          = errors.New("could not verify server")
	ErrFailedFetch        = errors.New("failed to fetch data")
	ErrBadResponse        = errors.New("bad response from server")
	ErrBadSessionDuration = errors.New("session duration is less than 60 seconds")
	ErrFailedRefresh      = errors.New("failed to refresh session")
)

// Firefly provides a simplified client for BlueSky/AtProto with automatic session management.
// It handles JWT token refresh automatically and provides clean, Go-idiomatic interfaces
// for common BlueSky operations like searching posts and fetching notifications.
type Firefly struct {
	client            *xrpc.Client
	sessionExpiration time.Time
	cancelRefresh     context.CancelFunc

	// ErrorChan receives errors from background operations like token refresh.
	// Users should monitor this channel to handle authentication failures.
	ErrorChan chan error

	// Self contains the authenticated user's profile information, populated after Login().
	Self *User
}

// NewDefaultInstance creates a new Firefly client using the default BlueSky server (bsky.social)
// and a standard HTTP client. This is the recommended way to create a client for most users.
//
// Example:
//
//	client, err := firefly.NewDefaultInstance()
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewDefaultInstance() (*Firefly, error) {
	return NewCustomInstance(context.Background(), defaultBskyServer, new(http.Client))
}

// NewCustomInstance creates a new Firefly client with custom configuration.
// This allows you to specify a different AtProto server, custom HTTP client, or context.
// The server parameter should be a full URL (e.g., "https://bsky.social").
// Returns an error if the server cannot be reached or verified.
//
// Example:
//
//	ctx := context.WithTimeout(context.Background(), 30*time.Second)
//	client := &http.Client{Timeout: 10 * time.Second}
//	firefly, err := firefly.NewCustomInstance(ctx, "https://bsky.social", client)
func NewCustomInstance(ctx context.Context, server string, client *http.Client) (*Firefly, error) {
	local := &xrpc.Client{
		Client: client,
		Host:   server,
	}
	if _, err := atproto.ServerDescribeServer(ctx, local); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadServer, err)
	}

	return &Firefly{
		client:        local,
		ErrorChan:     make(chan error),
		cancelRefresh: nil,
	}, nil
}

// Login authenticates with BlueSky using username (handle) and password.
// It automatically schedules JWT token refresh and populates the Self field with user profile information.
// The username can be either a handle (e.g., "alice.bsky.social") or email address.
//
// Example:
//
//	err := client.Login(context.Background(), "alice.bsky.social", "my-app-password")
//	if err != nil {
//	    log.Fatal("Login failed:", err)
//	}
//	fmt.Printf("Logged in as: %s\n", client.Self.DisplayName)
func (f *Firefly) Login(ctx context.Context, username string, password string) error {
	if _, err := atproto.ServerDescribeServer(ctx, f.client); err != nil {
		return fmt.Errorf("%w: %w", ErrBadServer, err)
	}
	authInput := atproto.ServerCreateSession_Input{
		Identifier: username,
		Password:   password,
	}
	authOutput, err := atproto.ServerCreateSession(ctx, f.client, &authInput)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBadLogin, err)
	}

	authToken, _, err := jwt.NewParser().ParseUnverified(authOutput.AccessJwt, jwt.MapClaims{})
	if authToken == nil || (err != nil && !errors.Is(err, jwt.ErrTokenUnverifiable)) {
		return fmt.Errorf("%w: %w", ErrBadResponse, err)
	}
	expDate, err := authToken.Claims.GetExpirationTime()
	if expDate == nil || err != nil {
		return fmt.Errorf("%w: %w", ErrBadResponse, err)
	}

	f.sessionExpiration = expDate.Time
	if f.sessionExpiration.Sub(time.Now()).Seconds() < 60 {
		return ErrBadSessionDuration
	}

	f.client.Auth = &xrpc.AuthInfo{
		AccessJwt:  authOutput.AccessJwt,
		RefreshJwt: authOutput.RefreshJwt,
		Handle:     authOutput.Handle,
		Did:        authOutput.Did,
	}

	f.scheduleSessionRefresh()

	profile, err := bsky.ActorGetProfile(ctx, f.client, authOutput.Handle)
	if err == nil {
		selfUser, err := OldToNewDetailedUser(profile)
		if err == nil {
			f.Self = selfUser
		}
	}

	return nil
}

// updateSession refreshes the session tokens, updates expiration time, and checks the session duration for validity.
func (f *Firefly) updateSession(ctx context.Context) error {
	authOutput, err := atproto.ServerRefreshSession(ctx, f.client)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailedRefresh, err)
	}

	authToken, _, err := jwt.NewParser().ParseUnverified(authOutput.AccessJwt, jwt.MapClaims{})
	if authToken == nil || (err != nil && !errors.Is(err, jwt.ErrTokenUnverifiable)) {
		return fmt.Errorf("%w: %w", ErrFailedRefresh, err)
	}
	expDate, err := authToken.Claims.GetExpirationTime()
	if expDate == nil || err != nil {
		return fmt.Errorf("%w: %w", ErrFailedRefresh, err)
	}

	f.sessionExpiration = expDate.Time
	if f.sessionExpiration.Sub(time.Now()).Seconds() < 60 {
		return ErrBadSessionDuration
	}

	f.client.Auth = &xrpc.AuthInfo{
		AccessJwt:  authOutput.AccessJwt,
		RefreshJwt: authOutput.RefreshJwt,
		Handle:     authOutput.Handle,
		Did:        authOutput.Did,
	}

	return nil
}

// scheduleSessionRefresh schedules Firefly to refresh the session token 1 minute before expiration
func (f *Firefly) scheduleSessionRefresh() {
	refreshCtx, cancel := context.WithCancel(context.Background())
	f.cancelRefresh = cancel
	time.AfterFunc(f.sessionExpiration.Sub(time.Now().Add(time.Minute)), func() {
		select {
		case <-refreshCtx.Done():
			return
		default:
			// Create a context with timeout for the refresh operation
			ctx, cancelOp := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancelOp()

			err := f.updateSession(ctx)
			if err != nil {
				f.ErrorChan <- err
				f.cancelRefresh = nil
			} else {
				f.scheduleSessionRefresh()
			}
		}
	})
}

// RefreshSession manually refreshes the authentication token before its scheduled expiration.
// This cancels any existing refresh timer and schedules a new one.
// Any errors during refresh are sent to ErrorChan rather than returned.
//
// This is typically not needed as Firefly handles token refresh automatically,
// but can be useful if you suspect the token is invalid or want to refresh proactively.
func (f *Firefly) RefreshSession(ctx context.Context) {
	if f.cancelRefresh != nil {
		f.cancelRefresh()
	}
	err := f.updateSession(ctx)
	if err != nil {
		f.ErrorChan <- err
		f.cancelRefresh = nil
	} else {
		f.scheduleSessionRefresh()
	}
	return
}
