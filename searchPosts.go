package firefly

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/api/bsky"
)

var (
	ErrSearchFailed = errors.New("search failed")
)

type SortOrder string

// Search sort options
const (
	// SortByTop sorts search results by engagement (likes, reposts, replies)
	SortByTop SortOrder = "top"
	// SortByLatest sorts search results by creation time, newest first
	SortByLatest SortOrder = "latest"
)

// PostSearch holds all search filter options for post searching
type PostSearch struct {
	Author   string     // Filter by author handle or DID
	Cursor   string     // Pagination cursor
	Domain   string     // Filter by domain
	Language string     // Filter by language code (e.g., "en", "es")
	Mentions string     // Filter posts mentioning a specific user
	SortBy   SortOrder  // Sort order ("top" or "latest")
	URL      string     // Filter posts containing a specific URL
	Tags     []string   // Filter by hashtags
	From     *time.Time // Posts after this time
	Until    *time.Time // Posts before this time
}

// SearchPosts searches for posts with optional filters.
// Pass nil for options to search without filters.
func (f *Firefly) SearchPosts(ctx context.Context, query string, limit int, options *PostSearch) ([]*FeedPost, error) {
	if options == nil {
		options = &PostSearch{}
	}

	var posts []*FeedPost

	fromTime := ""
	if options.From != nil {
		fromTime = options.From.Format(time.RFC3339)
	}
	toTime := ""
	if options.Until != nil {
		toTime = options.Until.Format(time.RFC3339)
	}
	results, err := bsky.FeedSearchPosts(
		ctx, f.client, options.Author, options.Cursor,
		options.Domain, options.Language, int64(limit),
		options.Mentions, query, fromTime, string(options.SortBy),
		options.Tags, toTime, options.URL)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSearchFailed, err)
	}
	if results == nil {
		return nil, fmt.Errorf("%w: %w", ErrSearchFailed, errors.New("nil results returned"))
	}
	posts = make([]*FeedPost, len(results.Posts))
	for i, postView := range results.Posts {
		newPost, err := f.OldToNewPostView(postView)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrSearchFailed, err)
		} else {
			posts[i] = newPost
		}
	}

	return posts, nil
}
