package firefly

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

var (
	ErrEmptyUri   = errors.New("empty URI")
	ErrInvalidUri = errors.New("invalid URI")
	ErrNoDid      = errors.New("URI uses a handle, not a DID")
)

// ExtractDidFromUri extracts the DID from an AT URI format: at://did:plc:xyz123/collection/record
// if URI is like at://an.example.handle/app.bsky.feed.post/, returns handle and ErrNoDid
func ExtractDidFromUri(uri string) (string, error) {
	if uri == "" {
		return "", ErrEmptyUri
	}

	_, err := syntax.ParseATURI(uri)
	if err != nil {
		return "", ErrInvalidUri
	}

	// AT URIs have format: at://did:plc:xyz123/app.bsky.feed.post/abc123
	if !strings.HasPrefix(uri, "at://") {
		return "", ErrInvalidUri
	}

	// Remove at:// prefix
	userID := uri[5:]
	// crop to the end of the id
	// technically, at://an.example.handle is valid, so we have to account for that
	end := strings.Index(userID, "/")
	if end > -1 {
		userID = userID[:end]
	}

	// AtProto URIs can be either DID based or handle based. If it's already a DID, return it
	if strings.HasPrefix(userID, "did") {
		return userID, nil
	}

	// if not, return it still but with an error so they know it's not a DID
	return userID, ErrNoDid
}

// ResolveHandleToDID resolves a BlueSky handle to its corresponding DID using the XRPC API
func (f *Firefly) ResolveHandleToDID(handle string) (string, error) {
	output, err := atproto.IdentityResolveHandle(f.ctx, f.client, handle)
	if err != nil {
		return "", fmt.Errorf("failed to resolve handle to DID: %w", err)
	}
	return output.Did, nil
}

// ExtractOrResolveDidFromUri extracts a DID from an AT URI, resolving handles to DIDs when necessary
func (f *Firefly) ExtractOrResolveDidFromUri(uri string) (string, error) {
	userID, err := ExtractDidFromUri(uri)
	if err != nil {
		if errors.Is(err, ErrNoDid) {
			// userID is a handle, resolve it to a DID
			return f.ResolveHandleToDID(userID)
		}
		// Other error (empty URI, invalid format, etc.)
		return "", err
	}
	// userID is already a DID
	return userID, nil
}
