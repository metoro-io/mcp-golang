package mcp_golang

import (
	"encoding/json"
	"fmt"

	"github.com/tidwall/sjson"
)

type Role string

const RoleAssistant Role = "assistant"
const RoleUser Role = "user"

type Annotations struct {
	// Describes who the intended customer of this object or data is.
	//
	// It can include multiple entries to indicate ToolResponse useful for multiple
	// audiences (e.g., `["user", "assistant"]`).
	Audience []Role `json:"audience,omitempty" yaml:"audience,omitempty" mapstructure:"audience,omitempty"`

	// Describes how important this data is for operating the server.
	//
	// A value of 1 means "most important," and indicates that the data is
	// effectively required, while 0 means "least important," and indicates that
	// the data is entirely optional.
	Priority *float64 `json:"priority,omitempty" yaml:"priority,omitempty" mapstructure:"priority,omitempty"`
}

// Text provided to or from an LLM.
type TextContent struct {
	// The text ToolResponse of the message.
	Text string `json:"text" yaml:"text" mapstructure:"text"`
}

// An image provided to or from an LLM.
type ImageContent struct {
	// The base64-encoded image data.
	Data string `json:"data" yaml:"data" mapstructure:"data"`

	// The MIME type of the image. Different providers may support different image
	// types.
	MimeType string `json:"mimeType" yaml:"mimeType" mapstructure:"mimeType"`
}

type embeddedResourceType string

const (
	embeddedResourceTypeBlob embeddedResourceType = "blob"
	embeddedResourceTypeText embeddedResourceType = "text"
)

type BlobResourceContents struct {
	// A base64-encoded string representing the binary data of the item.
	Blob string `json:"blob" yaml:"blob" mapstructure:"blob"`

	// The MIME type of this resource, if known.
	MimeType *string `json:"mimeType,omitempty" yaml:"mimeType,omitempty" mapstructure:"mimeType,omitempty"`

	// The URI of this resource.
	Uri string `json:"uri" yaml:"uri" mapstructure:"uri"`
}

type TextResourceContents struct {
	// The MIME type of this resource, if known.
	MimeType *string `json:"mimeType,omitempty" yaml:"mimeType,omitempty" mapstructure:"mimeType,omitempty"`

	// The text of the item. This must only be set if the item can actually be
	// represented as text (not binary data).
	Text string `json:"text" yaml:"text" mapstructure:"text"`

	// The URI of this resource.
	Uri string `json:"uri" yaml:"uri" mapstructure:"uri"`
}

// The contents of a resource, embedded into a prompt or tool call result.
//
// It is up to the client how best to render embedded resources for the benefit
// of the LLM and/or the user.
type EmbeddedResource struct {
	EmbeddedResourceType embeddedResourceType
	TextResourceContents *TextResourceContents
	BlobResourceContents *BlobResourceContents
}

// Custom JSON marshaling for EmbeddedResource
func (c EmbeddedResource) MarshalJSON() ([]byte, error) {
	switch c.EmbeddedResourceType {
	case embeddedResourceTypeBlob:
		return json.Marshal(c.BlobResourceContents)
	case embeddedResourceTypeText:
		return json.Marshal(c.TextResourceContents)
	default:
		return nil, fmt.Errorf("unknown embedded resource type: %s", c.EmbeddedResourceType)
	}
}

// Custom JSON unmarshaling for EmbeddedResource
func (c *EmbeddedResource) UnmarshalJSON(data []byte) error {
	// First try to unmarshal as TextResourceContents
	var textResource TextResourceContents
	err := json.Unmarshal(data, &textResource)
	if err == nil && textResource.Text != "" {
		c.EmbeddedResourceType = embeddedResourceTypeText
		c.TextResourceContents = &textResource
		return nil
	}

	// Then try to unmarshal as BlobResourceContents
	var blobResource BlobResourceContents
	err = json.Unmarshal(data, &blobResource)
	if err == nil && blobResource.Blob != "" {
		c.EmbeddedResourceType = embeddedResourceTypeBlob
		c.BlobResourceContents = &blobResource
		return nil
	}

	return fmt.Errorf("failed to unmarshal embedded resource: %v", err)
}

type ContentType string

const (
	// The value is the value of the "type" field in the Content so do not change
	ContentTypeText             ContentType = "text"
	ContentTypeImage            ContentType = "image"
	ContentTypeEmbeddedResource ContentType = "resource"
)

type Content struct {
	Type             ContentType
	TextContent      *TextContent
	ImageContent     *ImageContent
	EmbeddedResource *EmbeddedResource
	Annotations      *Annotations
}

func (c *Content) UnmarshalJSON(b []byte) error {
	type typeWrapper struct {
		Type             ContentType       `json:"type" yaml:"type" mapstructure:"type"`
		Text             *string           `json:"text" yaml:"text" mapstructure:"text"`
		Image            *string           `json:"image" yaml:"image" mapstructure:"image"`
		Annotations      *Annotations      `json:"annotations" yaml:"annotations" mapstructure:"annotations"`
		EmbeddedResource *EmbeddedResource `json:"resource" yaml:"resource" mapstructure:"resource"`
	}
	var tw typeWrapper
	err := json.Unmarshal(b, &tw)
	if err != nil {
		return err
	}
	switch tw.Type {
	case ContentTypeText:
		c.Type = ContentTypeText
	case ContentTypeImage:
		c.Type = ContentTypeImage
	case ContentTypeEmbeddedResource:
		c.Type = ContentTypeEmbeddedResource
	default:
		return fmt.Errorf("unknown content type: %s", tw.Type)
	}

	switch c.Type {
	case ContentTypeText:
		c.TextContent = &TextContent{Text: *tw.Text}
	default:
		return fmt.Errorf("unknown content type: %s", c.Type)
	}

	return nil
}

// Custom JSON marshaling for ToolResponse Content
func (c Content) MarshalJSON() ([]byte, error) {
	var rawJson []byte

	switch c.Type {
	case ContentTypeText:
		j, err := json.Marshal(c.TextContent)
		if err != nil {
			return nil, err
		}
		rawJson = j
	case ContentTypeImage:
		j, err := json.Marshal(c.ImageContent)
		if err != nil {
			return nil, err
		}
		rawJson = j
	case ContentTypeEmbeddedResource:
		j, err := json.Marshal(c.EmbeddedResource)
		if err != nil {
			return nil, err
		}
		rawJson = j
	default:
		return nil, fmt.Errorf("unknown content type: %s", c.Type)
	}

	// Add the type
	rawJson, err := sjson.SetBytes(rawJson, "type", string(c.Type))
	if err != nil {
		return nil, err
	}
	// Add the annotations
	if c.Annotations != nil {
		marshal, err := json.Marshal(c.Annotations)
		if err != nil {
			return nil, err
		}
		rawJson, err = sjson.SetBytes(rawJson, "annotations", marshal)
		if err != nil {
			return nil, err
		}
	}
	return rawJson, nil
}

func (c *Content) WithAnnotations(annotations Annotations) *Content {
	c.Annotations = &annotations
	return c
}

// newToolResponseSentError creates a new ToolResponse that represents an error.
// This is used to create a result that will be returned to the client as an error for a tool call.
func newToolResponseSentError(err error) *toolResponseSent {
	return &toolResponseSent{
		Error: err,
	}
}

// newToolResponseSent creates a new toolResponseSent
func newToolResponseSent(response *ToolResponse) *toolResponseSent {
	return &toolResponseSent{
		Response: response,
	}
}

// NewImageContent creates a new ToolResponse that is an image.
// The given data is base64-encoded
func NewImageContent(base64EncodedStringData string, mimeType string) *Content {
	return &Content{
		Type:         ContentTypeImage,
		ImageContent: &ImageContent{Data: base64EncodedStringData, MimeType: mimeType},
	}
}

// NewTextContent creates a new ToolResponse that is a simple text string.
// The client will render this as a single string.
func NewTextContent(content string) *Content {
	return &Content{
		Type:        ContentTypeText,
		TextContent: &TextContent{Text: content},
	}
}

// NewBlobResourceContent creates a new ToolResponse that is a blob of binary data.
// The given data is base64-encoded; the client will decode it.
// The client will render this as a blob; it will not be human-readable.
func NewBlobResourceContent(uri string, base64EncodedData string, mimeType string) *Content {
	return &Content{
		Type: ContentTypeEmbeddedResource,
		EmbeddedResource: &EmbeddedResource{
			EmbeddedResourceType: embeddedResourceTypeBlob,
			BlobResourceContents: &BlobResourceContents{
				Blob:     base64EncodedData,
				MimeType: &mimeType,
				Uri:      uri,
			}},
	}
}

// NewTextResourceContent creates a new ToolResponse that is an embedded resource of type "text".
// The given text is embedded in the response as a TextResourceContents, which
// contains the given MIME type and URI. The text is not base64-encoded.
func NewTextResourceContent(uri string, text string, mimeType string) *Content {
	return &Content{
		Type: ContentTypeEmbeddedResource,
		EmbeddedResource: &EmbeddedResource{
			EmbeddedResourceType: embeddedResourceTypeText,
			TextResourceContents: &TextResourceContents{
				MimeType: &mimeType, Text: text, Uri: uri,
			}},
	}
}

func NewTextEmbeddedResource(uri string, text string, mimeType string) *EmbeddedResource {
	return &EmbeddedResource{
		EmbeddedResourceType: embeddedResourceTypeText,
		TextResourceContents: &TextResourceContents{
			MimeType: &mimeType, Text: text, Uri: uri,
		}}
}

func NewBlobEmbeddedResource(uri string, base64EncodedData string, mimeType string) *EmbeddedResource {
	return &EmbeddedResource{
		EmbeddedResourceType: embeddedResourceTypeBlob,
		BlobResourceContents: &BlobResourceContents{
			Blob:     base64EncodedData,
			MimeType: &mimeType,
			Uri:      uri,
		}}
}
