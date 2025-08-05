package firefly

import "github.com/bluesky-social/indigo/api/bsky"

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
