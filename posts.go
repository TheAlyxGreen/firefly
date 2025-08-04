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

// EmbedType identifies the type of embedded content in a post.
type EmbedType int

const (
	EmbedTypeUnknown EmbedType = iota
	EmbedTypeImages
	EmbedTypeExternal
	EmbedTypeRecord
	EmbedTypeVideo
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

func (et EmbedType) String() string {
	switch et {
	case EmbedTypeImages:
		return "Images"
	case EmbedTypeExternal:
		return "External Link"
	case EmbedTypeRecord:
		return "Quote Post"
	case EmbedTypeVideo:
		return "Video"
	default:
		return "Unknown Embed"
	}
}

// EmbedImage represents an image embedded in a post.
type EmbedImage struct {
	AltText string `json:"altText" cborgen:"altText"`
	URL     string `json:"url" cborgen:"url"`
}

// EmbedLink represents an external link embedded in a post.
type EmbedLink struct {
	URL         string `json:"url" cborgen:"url"`
	Title       string `json:"title" cborgen:"title"`
	Description string `json:"description" cborgen:"description"`
	ThumbURL    string `json:"thumbUrl,omitempty" cborgen:"thumbUrl,omitempty"`
}

// EmbedVideo represents a video embedded in a post.
type EmbedVideo struct {
	URL     string `json:"url" cborgen:"url"`
	AltText string `json:"altText,omitempty" cborgen:"altText,omitempty"`
}

// Embed represents embedded content in a post with a simplified, flattened structure.
type Embed struct {
	Type     EmbedType            `json:"type" cborgen:"type"`
	Images   []EmbedImage         `json:"images,omitempty" cborgen:"images,omitempty"`
	External *EmbedLink           `json:"external,omitempty" cborgen:"external,omitempty"`
	Record   *PostRef             `json:"record,omitempty" cborgen:"record,omitempty"`
	Video    *EmbedVideo          `json:"video,omitempty" cborgen:"video,omitempty"`
	Raw      *bsky.FeedPost_Embed `json:"-" cborgen:"-"`
}

func (e Embed) String() string {
	switch e.Type {
	case EmbedTypeImages:
		return fmt.Sprintf("Embed{Type: %s, Images: %d}", e.Type, len(e.Images))
	case EmbedTypeExternal:
		if e.External != nil {
			return fmt.Sprintf("Embed{Type: %s, URL: %s}", e.Type, e.External.URL)
		}
		return fmt.Sprintf("Embed{Type: %s}", e.Type)
	case EmbedTypeRecord:
		if e.Record != nil {
			return fmt.Sprintf("Embed{Type: %s, URI: %s}", e.Type, e.Record.Uri)
		}
		return fmt.Sprintf("Embed{Type: %s}", e.Type)
	case EmbedTypeVideo:
		if e.Video != nil {
			return fmt.Sprintf("Embed{Type: %s, URL: %s}", e.Type, e.Video.URL)
		}
		return fmt.Sprintf("Embed{Type: %s}", e.Type)
	default:
		return fmt.Sprintf("Embed{Type: %s}", e.Type)
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

// OldToNewEmbed converts BlueSky's complex embed types to Firefly's simplified Embed structure
func (f *Firefly) OldToNewEmbed(oldEmbed *bsky.FeedPost_Embed, authorDID string) (*Embed, error) {
	if oldEmbed == nil {
		return nil, nil
	}

	embed := &Embed{
		Type: EmbedTypeUnknown,
		Raw:  oldEmbed,
	}

	// Handle EmbedImages
	if oldEmbed.EmbedImages != nil {
		embed.Type = EmbedTypeImages
		embed.Images = make([]EmbedImage, len(oldEmbed.EmbedImages.Images))

		for i, img := range oldEmbed.EmbedImages.Images {
			imageURL := ""
			if img.Image != nil && img.Image.Ref.String() != "" {
				// Construct blob URL: https://server/xrpc/com.atproto.sync.getBlob?did=userDID&cid=blobCID
				imageURL = fmt.Sprintf("%s/xrpc/com.atproto.sync.getBlob?did=%s&cid=%s",
					f.client.Host, authorDID, img.Image.Ref.String())
			}
			embed.Images[i] = EmbedImage{
				AltText: img.Alt,
				URL:     imageURL,
			}
		}
	}

	// Handle EmbedExternal
	if oldEmbed.EmbedExternal != nil && oldEmbed.EmbedExternal.External != nil {
		embed.Type = EmbedTypeExternal
		thumbURL := ""
		if oldEmbed.EmbedExternal.External.Thumb != nil && oldEmbed.EmbedExternal.External.Thumb.Ref.String() != "" {
			// Construct blob URL for thumbnail
			thumbURL = fmt.Sprintf("%s/xrpc/com.atproto.sync.getBlob?did=%s&cid=%s",
				f.client.Host, authorDID, oldEmbed.EmbedExternal.External.Thumb.Ref.String())
		}
		embed.External = &EmbedLink{
			URL:         oldEmbed.EmbedExternal.External.Uri,
			Title:       oldEmbed.EmbedExternal.External.Title,
			Description: oldEmbed.EmbedExternal.External.Description,
			ThumbURL:    thumbURL,
		}
	}

	// Handle EmbedRecord (quote posts)
	if oldEmbed.EmbedRecord != nil && oldEmbed.EmbedRecord.Record != nil {
		embed.Type = EmbedTypeRecord
		embed.Record = &PostRef{
			Cid: oldEmbed.EmbedRecord.Record.Cid,
			Uri: oldEmbed.EmbedRecord.Record.Uri,
		}
	}

	// Handle EmbedVideo
	if oldEmbed.EmbedVideo != nil {
		embed.Type = EmbedTypeVideo
		videoURL := ""
		if oldEmbed.EmbedVideo.Video != nil && oldEmbed.EmbedVideo.Video.Ref.String() != "" {
			// Construct blob URL for video
			videoURL = fmt.Sprintf("%s/xrpc/com.atproto.sync.getBlob?did=%s&cid=%s",
				f.client.Host, authorDID, oldEmbed.EmbedVideo.Video.Ref.String())
		}
		altText := ""
		if oldEmbed.EmbedVideo.Alt != nil {
			altText = *oldEmbed.EmbedVideo.Alt
		}
		embed.Video = &EmbedVideo{
			URL:     videoURL,
			AltText: altText,
		}
	}

	// Handle EmbedRecordWithMedia (combination)
	if oldEmbed.EmbedRecordWithMedia != nil {
		// This is complex - it contains both a record and media
		// For simplicity, we'll treat it as a record type and note the media in Raw
		embed.Type = EmbedTypeRecord
		if oldEmbed.EmbedRecordWithMedia.Record != nil && oldEmbed.EmbedRecordWithMedia.Record.Record != nil {
			embed.Record = &PostRef{
				Cid: oldEmbed.EmbedRecordWithMedia.Record.Record.Cid,
				Uri: oldEmbed.EmbedRecordWithMedia.Record.Record.Uri,
			}
		}
	}

	return embed, nil
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
	newPost, err := f.OldToNewPost(oldPost, oldPostView.Author.Did)
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

func (p FeedPost) String() string {
	timestamp := p.CreatedAt.Format("02 Jan 2006 @ 15:04")
	safeText := strings.Replace(p.Text, "\n", "\\n", -1)
	replyText := ""
	if p.ReplyInfo != nil {
		replyText = ", ReplyTo: " + p.ReplyInfo.ReplyTarget.Uri
	}
	return fmt.Sprintf("FeedPost{Timestamp: %s, Text: '%s%s'}", timestamp, safeText, replyText)
}
