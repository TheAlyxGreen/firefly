package firefly

import (
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

// postSearchConfig holds all the optional parameters for post searching
type postSearchConfig struct {
	author   string
	cursor   string
	domain   string
	lang     string
	mentions string
	sort     SortOrder
	url      string
	tags     []string
	from     *time.Time
	until    *time.Time
	limit    int
}

// PostSearchOption configures post search parameters
type PostSearchOption func(*postSearchConfig)

// SearchByAuthor filters posts by a specific author handle or DID
func SearchByAuthor(author string) PostSearchOption {
	return func(c *postSearchConfig) { c.author = author }
}

// SearchWithCursor sets the cursor for pagination
func SearchWithCursor(cursor string) PostSearchOption {
	return func(c *postSearchConfig) { c.cursor = cursor }
}

// SearchByDomain filters posts by domain
func SearchByDomain(domain string) PostSearchOption {
	return func(c *postSearchConfig) { c.domain = domain }
}

// SearchByLanguage filters posts by language code (e.g., "en", "es")
func SearchByLanguage(lang string) PostSearchOption {
	return func(c *postSearchConfig) { c.lang = lang }
}

// SearchByMentions filters posts mentioning a specific user handle or DID
func SearchByMentions(mentions string) PostSearchOption {
	return func(c *postSearchConfig) { c.mentions = mentions }
}

// SearchSortBy sets the sort order ("top" or "latest")
func SearchSortBy(sort SortOrder) PostSearchOption {
	return func(c *postSearchConfig) { c.sort = sort }
}

// SearchByURL filters posts containing a specific URL
func SearchByURL(url string) PostSearchOption {
	return func(c *postSearchConfig) { c.url = url }
}

// SearchByTags filters posts by hashtags
func SearchByTags(tags ...string) PostSearchOption {
	return func(c *postSearchConfig) { c.tags = tags }
}

// SearchTimeRange filters posts within a time range
func SearchTimeRange(from, until *time.Time) PostSearchOption {
	return func(c *postSearchConfig) {
		c.from = from
		c.until = until
	}
}

// SearchFrom filters posts created after this time
func SearchFrom(from time.Time) PostSearchOption {
	return func(c *postSearchConfig) { c.from = &from }
}

// SearchUntil filters posts created before this time
func SearchUntil(until time.Time) PostSearchOption {
	return func(c *postSearchConfig) { c.until = &until }
}

// SearchLimit sets the maximum number of posts to return (1-100, default 25)
func SearchLimit(limit int) PostSearchOption {
	return func(c *postSearchConfig) { c.limit = limit }
}

// advancedPostSearch lets you search Bluesky with all the parameters/filters it supports
func (f *Firefly) advancedPostSearch(query, author, cursor, domain, lang, mentions, url string, sort SortOrder, tags []string, from, until *time.Time, limit int) ([]*FeedPost, error) {
	var posts []*FeedPost
	
	fromTime := ""
	if from != nil {
		fromTime = from.Format(time.RFC3339)
	}
	toTime := ""
	if until != nil {
		toTime = until.Format(time.RFC3339)
	}
	results, err := bsky.FeedSearchPosts(f.ctx, f.client, author, cursor, domain, lang, int64(limit), mentions, query, fromTime, string(sort), tags, toTime, url)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSearchFailed, err)
	}
	if results == nil {
		return nil, fmt.Errorf("%w: %w", ErrSearchFailed, errors.New("nil results returned"))
	}
	posts = make([]*FeedPost, len(results.Posts))
	for i, postView := range results.Posts {
		newPost, err := OldToNewPost(postView.Record.Val.(*bsky.FeedPost))
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrSearchFailed, err)
		} else {
			posts[i] = newPost
		}
	}
	
	return posts, nil
}

// SearchPosts searches for posts using the functional options pattern for clean, readable API calls
func (f *Firefly) SearchPosts(query string, opts ...PostSearchOption) ([]*FeedPost, error) {
	config := &postSearchConfig{
		limit: 25, // sensible default
	}
	
	for _, opt := range opts {
		opt(config)
	}
	
	return f.advancedPostSearch(
		query,
		config.author,
		config.cursor,
		config.domain,
		config.lang,
		config.mentions,
		config.url,
		config.sort,
		config.tags,
		config.from,
		config.until,
		config.limit,
	)
}

// SimplePostSearch is a convenience wrapper for basic searches
func (f *Firefly) SimplePostSearch(query string, limit int) ([]*FeedPost, error) {
	return f.SearchPosts(query, SearchLimit(limit))
}
