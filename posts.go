package firefly

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

var (
	ErrNilFacet     = errors.New("nil facet")
	ErrNilPost      = errors.New("nil post")
	ErrInvalidFacet = errors.New("invalid facet")
	ErrInvalidPost  = errors.New("invalid post")
)

// PostRef provides a content-addressed reference to a BlueSky post.
// The Uri points to the post's location, while the Cid is a cryptographic hash of the post content.
type PostRef struct {
	Cid string `json:"cid" cborgen:"cid"` // hash of the content of the post
	Uri string `json:"uri" cborgen:"uri"` // pointer to the location of the post
}

// IsValid validates the format of both the Cid and Uri. It does not check if they actually work/exist, just that they
// are formatted correctly
func (ref *PostRef) IsValid() bool {
	_, err := syntax.ParseCID(ref.Cid)
	if err != nil {
		return false
	}
	_, err = syntax.ParseATURI(ref.Uri)
	if err != nil {
		return false
	}
	return true
}

// OldToNewRefPointer converts a pointer to the old reference to a pointer to the new reference or nil
func OldToNewRefPointer(oldRef *atproto.RepoStrongRef) *PostRef {
	if oldRef == nil {
		return nil
	}
	return &PostRef{
		Cid: oldRef.Cid,
		Uri: oldRef.Uri,
	}
}

// ReplyInfo contains thread information for posts that are replies.
// ReplyTarget is the immediate parent post, while ReplyRoot is the top-level post in the thread.
type ReplyInfo struct { // nil if not a reply
	ReplyTarget *PostRef `json:"replyTarget" cborgen:"replyTarget"` // post that this post is replying to
	ReplyRoot   *PostRef `json:"replyRoot" cborgen:"replyRoot"`     // top-level post the ReplyTarget is under
}

// FeedPost represents a BlueSky post with all its content and metadata.
// This includes the post text, rich text formatting, creation time, language, and thread information.
// Some fields like Uri, Cid, and Author may be populated depending on the context where the post was retrieved.
type FeedPost struct {
	Uri         string          `json:"uri" cborgen:"uri"`       // may be empty
	Cid         string          `json:"cid" cborgen:"cid"`       // may be empty
	Author      *User           `json:"author" cborgen:"author"` // may be nil
	CreatedAt   *time.Time      `json:"createdAt" cborgen:"createdAt"`
	IndexedAt   *time.Time      `json:"indexedAt" cborgen:"indexedAt"`
	Facets      []RichTextFacet `json:"facets" cborgen:"facets"`
	Text        string          `json:"text" cborgen:"text"`
	Tags        []string        `json:"tags" cborgen:"tags"`
	Languages   []string        `json:"languages" cborgen:"languages"`
	ReplyInfo   *ReplyInfo      `json:"replyInfo" cborgen:"replyInfo"`
	LikeCount   *int            `json:"likeCount" cborgen:"likeCount"`
	QuoteCount  *int            `json:"quoteCount" cborgen:"quoteCount"`
	ReplyCount  *int            `json:"replyCount" cborgen:"replyCount"`
	RepostCount *int            `json:"repostCount" cborgen:"repostCount"`
	Embed       *Embed          `json:"embed,omitempty" cborgen:"embed,omitempty"`
	Raw         *bsky.FeedPost
	RawDetailed *bsky.FeedDefs_PostView
	//Labels        []*comatprototypes.LabelDefs_Label `json:"labels,omitempty" cborgen:"labels,omitempty"`
	//Threadgate    *FeedDefs_ThreadgateView           `json:"threadgate,omitempty" cborgen:"threadgate,omitempty"`
	//Viewer        *FeedDefs_ViewerState              `json:"viewer,omitempty" cborgen:"viewer,omitempty"`
}

func (p FeedPost) String() string {
	timestamp := p.CreatedAt.Format("02 Jan 2006 @ 15:04")
	safeText := strings.Replace(p.Text, "\n", "\\n", -1)
	replyText := ""
	if p.ReplyInfo != nil {
		replyText = ", ReplyTo: " + p.ReplyInfo.ReplyTarget.Uri
	}
	return fmt.Sprintf("FeedPost{Timestamp: %s, Text: '%s%s'}", timestamp, safeText, replyText)
}

// OldToNewPost converts bsky posts into Firefly FeedPost types
func (f *Firefly) OldToNewPost(oldPost *bsky.FeedPost, authorDID string) (*FeedPost, error) {
	if oldPost == nil {
		return nil, ErrNilPost
	}

	CreatedAt, err := time.Parse(time.RFC3339, oldPost.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPost, err)
	}
	var NewFacets []RichTextFacet
	for _, facet := range oldPost.Facets {
		if facet != nil {
			newFacet, err := OldToNewFacet(facet)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", ErrInvalidPost, err)
			} else {
				NewFacets = append(NewFacets, *newFacet)
			}
		}
	}
	var NewReplyInfo *ReplyInfo
	if oldPost.Reply != nil && oldPost.Reply.Parent != nil {
		NewReplyInfo = &ReplyInfo{
			ReplyTarget: &PostRef{
				Cid: oldPost.Reply.Parent.Cid,
				Uri: oldPost.Reply.Parent.Uri,
			},
			ReplyRoot: nil,
		}
		if oldPost.Reply.Root != nil {
			NewReplyInfo.ReplyRoot = &PostRef{
				Cid: oldPost.Reply.Root.Cid,
				Uri: oldPost.Reply.Root.Uri,
			}
		}
	}

	// Convert embed if present
	newEmbed, err := f.OldToNewEmbed(oldPost.Embed, authorDID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPost, err)
	}

	newPost := &FeedPost{
		CreatedAt: &CreatedAt,
		Facets:    NewFacets,
		Text:      oldPost.Text,
		ReplyInfo: NewReplyInfo,
		Languages: oldPost.Langs,
		Tags:      oldPost.Tags,
		Embed:     newEmbed,
		Raw:       oldPost,
	}
	return newPost, nil
}

func (f *Firefly) OldToNewPostView(oldPostView *bsky.FeedDefs_PostView) (*FeedPost, error) {
	if oldPostView == nil {
		return nil, ErrNilPost
	}
	oldPost := oldPostView.Record.Val.(*bsky.FeedPost)
	newPost, err := f.OldToNewPost(oldPost, oldPostView.Uri)
	if err != nil {
		return nil, err
	}
	newPost.RawDetailed = oldPostView
	newPost.Uri = oldPostView.Uri
	newPost.Cid = oldPostView.Cid

	var likes int
	if oldPostView.LikeCount != nil {
		likes = int(*oldPostView.LikeCount)
	}
	newPost.LikeCount = &likes

	var quotes int
	if oldPostView.QuoteCount != nil {
		quotes = int(*oldPostView.QuoteCount)
	}
	newPost.QuoteCount = &quotes

	var replies int
	if oldPostView.ReplyCount != nil {
		replies = int(*oldPostView.ReplyCount)
	}
	newPost.ReplyCount = &replies

	var reposts int
	if oldPostView.RepostCount != nil {
		reposts = int(*oldPostView.RepostCount)
	}
	newPost.RepostCount = &reposts

	indexTime, err := time.Parse(time.RFC3339, oldPostView.IndexedAt)
	if err != nil {
		return newPost, fmt.Errorf("%w: %w", ErrInvalidPost, err)
	}
	newPost.IndexedAt = &indexTime
	newPost.Author, err = OldToNewUserBasic(oldPostView.Author)

	return newPost, err
}
