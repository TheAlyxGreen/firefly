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

// FacetType identifies the type of rich text element in a post (link, mention, hashtag, etc.).
type FacetType int

const (
	UnknownFacetType FacetType = iota
	LinkFacet
	MentionFacet
	TagFacet
)

func (ft FacetType) String() string {
	switch ft {
	case LinkFacet:
		return "Link Facet"
	case MentionFacet:
		return "Mention Facet"
	case TagFacet:
		return "Tag Facet"
	default:
		return "Unknown Facet"
	}
}

// RichTextFacet represents formatted text elements within a post such as links, mentions, and hashtags.
// It includes the type of element, its target (URL, user DID, hashtag), and position within the post text.
type RichTextFacet struct {
	Type       FacetType `json:"type" cborgen:"type"`
	Target     string    `json:"target" cborgen:"target"`
	StartIndex int       `json:"startIndex" cborgen:"startIndex"`
	EndIndex   int       `json:"endIndex" cborgen:"endIndex"`
}

// OldToNewFacet converts bsky facets into Firefly facets
func OldToNewFacet(oldFacet *bsky.RichtextFacet) (*RichTextFacet, error) {
	if oldFacet == nil {
		return nil, ErrNilFacet
	}
	if oldFacet.Features == nil || len(oldFacet.Features) != 1 {
		return nil, ErrInvalidFacet
	}
	newFacetType := UnknownFacetType
	target := ""
	if oldFacet.Features[0].RichtextFacet_Mention != nil {
		newFacetType = MentionFacet
		target = oldFacet.Features[0].RichtextFacet_Mention.Did
	} else if oldFacet.Features[0].RichtextFacet_Link != nil {
		newFacetType = LinkFacet
		target = oldFacet.Features[0].RichtextFacet_Link.Uri
	} else if oldFacet.Features[0].RichtextFacet_Tag != nil {
		newFacetType = TagFacet
		target = oldFacet.Features[0].RichtextFacet_Tag.Tag
	}
	newFacet := &RichTextFacet{
		Type:       newFacetType,
		Target:     target,
		StartIndex: int(oldFacet.Index.ByteStart),
		EndIndex:   int(oldFacet.Index.ByteEnd),
	}
	return newFacet, nil
}

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
	ReplyRoot   *PostRef `json:"replyParent" cborgen:"replyParent"` // top-level post the ReplyTarget is under
}

// FeedPost represents a BlueSky post with all its content and metadata.
// This includes the post text, rich text formatting, creation time, language, and thread information.
// Some fields like Uri, Cid, and Author may be populated depending on the context where the post was retrieved.
type FeedPost struct {
	Uri       string    `json:"uri" cborgen:"uri"`       // may be nil
	Cid       string    `json:"cid" cborgen:"cid"`       // may be nil
	Author    *User     `json:"author" cborgen:"author"` // may be nil
	CreatedAt time.Time `json:"createdAt" cborgen:"createdAt"`
	//Embed     *FeedPost_Embed `json:"embed,omitempty" cborgen:"embed,omitempty"`
	Facets    []RichTextFacet `json:"facets" cborgen:"facets"`
	Text      string          `json:"text" cborgen:"text"`
	Tags      []string        `json:"tags" cborgen:"tags"`
	Languages []string        `json:"languages" cborgen:"languages"`
	ReplyInfo *ReplyInfo      `json:"replyInfo" cborgen:"replyInfo"`
	Raw       *bsky.FeedPost
}

// OldToNewPost converts bsky posts into Firefly FeedPost types
func OldToNewPost(oldPost *bsky.FeedPost) (*FeedPost, error) {
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
	
	newPost := &FeedPost{
		CreatedAt: CreatedAt,
		Facets:    NewFacets,
		Text:      oldPost.Text,
		ReplyInfo: NewReplyInfo,
		Languages: oldPost.Langs,
		Tags:      oldPost.Tags,
		Raw:       oldPost,
	}
	return newPost, nil
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
