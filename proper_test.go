package firefly

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test actual functionality, not constants

func TestSearchPostsIntegration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.describeServer":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"did": "did:web:test.example",
			})
		case "/xrpc/app.bsky.feed.searchPosts":
			// Verify the functional options are properly converted to query params
			query := r.URL.Query()
			assert.Equal(t, "golang", query.Get("q"))
			assert.Equal(t, "test.bsky.social", query.Get("author"))
			assert.Equal(t, "en", query.Get("lang"))
			assert.Equal(t, "top", query.Get("sort"))
			assert.Equal(t, "50", query.Get("limit"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"posts": []map[string]interface{}{
					{
						"uri":    "at://did:plc:test/app.bsky.feed.post/123",
						"cid":    "bafkreitest123",
						"record": map[string]interface{}{
							"$type":     "app.bsky.feed.post",
							"text":      "Hello #golang world!",
							"createdAt": "2023-01-01T12:00:00Z",
							"facets":    []interface{}{},
							"langs":     []string{"en"},
							"tags":      []string{"golang"},
						},
						"author": map[string]interface{}{
							"did":         "did:plc:author123",
							"handle":      "author.bsky.social",
							"displayName": "Test Author",
							"createdAt":   "2022-01-01T12:00:00Z",
							"indexedAt":   "2022-01-01T12:30:00Z",
						},
					},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewCustomInstance(context.Background(), server.URL, &http.Client{})
	require.NoError(t, err)

	// Test that functional options actually work end-to-end
	posts, err := client.SearchPosts("golang",
		SearchByAuthor("test.bsky.social"),
		SearchByLanguage("en"),
		SearchSortBy(SortByTop),
		SearchLimit(50),
	)

	require.NoError(t, err)
	require.Len(t, posts, 1)

	post := posts[0]
	assert.Equal(t, "Hello #golang world!", post.Text)
	assert.Equal(t, []string{"en"}, post.Languages)
	assert.Equal(t, []string{"golang"}, post.Tags)

	expectedTime, _ := time.Parse(time.RFC3339, "2023-01-01T12:00:00Z")
	assert.Equal(t, expectedTime, post.CreatedAt)
}

func TestGetProfileIntegration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.describeServer":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"did": "did:web:test.example",
			})
		case "/xrpc/app.bsky.actor.getProfile":
			// Verify the handle parameter is passed correctly
			actor := r.URL.Query().Get("actor")
			assert.Equal(t, "test.bsky.social", actor)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"did":            "did:plc:test123",
				"handle":         "test.bsky.social",
				"displayName":    "Test User",
				"description":    "A test user profile",
				"avatar":         "https://example.com/avatar.jpg",
				"banner":         "https://example.com/banner.jpg",
				"followersCount": 1234,
				"followsCount":   567,
				"postsCount":     89,
				"createdAt":      "2023-01-01T00:00:00Z",
				"indexedAt":      "2023-01-01T00:30:00Z",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewCustomInstance(context.Background(), server.URL, &http.Client{})
	require.NoError(t, err)

	profile, err := client.GetProfile("test.bsky.social")
	require.NoError(t, err)
	require.NotNil(t, profile)

	// Test that the conversion actually works correctly
	assert.Equal(t, "did:plc:test123", profile.Did)
	assert.Equal(t, "test.bsky.social", profile.Handle)
	assert.Equal(t, "Test User", profile.DisplayName)
	assert.Equal(t, "A test user profile", profile.Description)
	assert.Equal(t, "https://example.com/avatar.jpg", profile.Avatar)
	assert.Equal(t, "https://example.com/banner.jpg", profile.Banner)
	assert.Equal(t, 1234, profile.FollowersCount)
	assert.Equal(t, 567, profile.FollowsCount)
	assert.Equal(t, 89, profile.PostsCount)

	// Test time parsing actually works
	expectedCreated, _ := time.Parse(time.RFC3339, "2023-01-01T00:00:00Z")
	expectedIndexed, _ := time.Parse(time.RFC3339, "2023-01-01T00:30:00Z")
	assert.Equal(t, expectedCreated, profile.CreatedAt)
	assert.Equal(t, expectedIndexed, profile.IndexedAt)
}

func TestSearchUsersIntegration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.describeServer":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"did": "did:web:test.example",
			})
		case "/xrpc/app.bsky.actor.searchActors":
			// Test that search options are correctly applied
			query := r.URL.Query()
			assert.Equal(t, "golang developer", query.Get("q"))
			assert.Equal(t, "10", query.Get("limit"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"actors": []map[string]interface{}{
					{
						"did":         "did:plc:dev123",
						"handle":      "godev.bsky.social",
						"displayName": "Go Developer",
						"description": "I love Go programming!",
						"createdAt":   "2023-01-01T12:00:00Z",
						"indexedAt":   "2023-01-01T12:30:00Z",
					},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewCustomInstance(context.Background(), server.URL, &http.Client{})
	require.NoError(t, err)

	users, err := client.SearchUsers("golang developer", UserSearchLimit(10))
	require.NoError(t, err)
	require.Len(t, users, 1)

	user := users[0]
	assert.Equal(t, "did:plc:dev123", user.Did)
	assert.Equal(t, "godev.bsky.social", user.Handle)
	assert.Equal(t, "Go Developer", user.DisplayName)
	assert.Equal(t, "I love Go programming!", user.Description)
}

func TestErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.describeServer":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"did": "did:web:test.example",
			})
		case "/xrpc/app.bsky.actor.getProfile":
			// Return 404 to test error handling
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "ProfileNotFound",
				"message": "Profile not found",
			})
		case "/xrpc/app.bsky.feed.searchPosts":
			// Return 400 to test search error handling
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "InvalidQuery",
				"message": "Invalid search query",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewCustomInstance(context.Background(), server.URL, &http.Client{})
	require.NoError(t, err)

	// Test GetProfile error handling
	profile, err := client.GetProfile("nonexistent.bsky.social")
	assert.Error(t, err)
	assert.Nil(t, profile)
	assert.Contains(t, err.Error(), "failed to fetch data")

	// Test SearchPosts error handling
	posts, err := client.SearchPosts("invalid query")
	assert.Error(t, err)
	assert.Nil(t, posts)
	assert.Contains(t, err.Error(), "search failed")
}

func TestSimplePostSearchUsesSearchPosts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.describeServer":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"did": "did:web:test.example",
			})
		case "/xrpc/app.bsky.feed.searchPosts":
			// Verify SimplePostSearch correctly maps to SearchPosts with limit
			query := r.URL.Query()
			assert.Equal(t, "test query", query.Get("q"))
			assert.Equal(t, "25", query.Get("limit"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"posts": []interface{}{},
			})
		}
	}))
	defer server.Close()

	client, err := NewCustomInstance(context.Background(), server.URL, &http.Client{})
	require.NoError(t, err)

	posts, err := client.SimplePostSearch("test query", 25)
	require.NoError(t, err)
	assert.NotNil(t, posts)
	assert.Len(t, posts, 0) // Empty result, but no error
}