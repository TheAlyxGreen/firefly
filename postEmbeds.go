package firefly

import (
	"fmt"

	"github.com/bluesky-social/indigo/api/bsky"
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
			return fmt.Sprintf("Embed{Type: %s, URI: %s}", e.Type, e.Record.URI)
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
			if img.Image != nil && img.Image.Ref.String() != "" && authorDID != "" {
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
		if oldEmbed.EmbedExternal.External.Thumb != nil && oldEmbed.EmbedExternal.External.Thumb.Ref.String() != "" && authorDID != "" {
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
			CID: oldEmbed.EmbedRecord.Record.Cid,
			URI: oldEmbed.EmbedRecord.Record.Uri,
		}
	}

	// Handle EmbedVideo
	if oldEmbed.EmbedVideo != nil {
		embed.Type = EmbedTypeVideo
		videoURL := ""
		if oldEmbed.EmbedVideo.Video != nil && oldEmbed.EmbedVideo.Video.Ref.String() != "" && authorDID != "" {
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
				CID: oldEmbed.EmbedRecordWithMedia.Record.Record.Cid,
				URI: oldEmbed.EmbedRecordWithMedia.Record.Record.Uri,
			}
		}
	}

	return embed, nil
}
