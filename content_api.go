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

// Custom JSON unmarshaling for EmbeddedResource. A "blob" key marks binary
// contents; everything else is text contents, including an empty text (which
// is valid and must not fail the whole response).
func (c *EmbeddedResource) UnmarshalJSON(data []byte) error {
	var probe struct {
		Blob *string `json:"blob"`
		Text *string `json:"text"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return fmt.Errorf("failed to unmarshal embedded resource: %v", err)
	}

	if probe.Blob != nil && probe.Text == nil {
		var blobResource BlobResourceContents
		if err := json.Unmarshal(data, &blobResource); err != nil {
			return fmt.Errorf("failed to unmarshal embedded resource: %v", err)
		}
		c.EmbeddedResourceType = embeddedResourceTypeBlob
		c.BlobResourceContents = &blobResource
		return nil
	}

	var textResource TextResourceContents
	if err := json.Unmarshal(data, &textResource); err != nil {
		return fmt.Errorf("failed to unmarshal embedded resource: %v", err)
	}
	c.EmbeddedResourceType = embeddedResourceTypeText
	c.TextResourceContents = &textResource
	return nil
}

type ContentType string

const (
	// The value is the value of the "type" field in the Content so do not change
	ContentTypeText             ContentType = "text"
	ContentTypeImage            ContentType = "image"
	ContentTypeAudio            ContentType = "audio"
	ContentTypeEmbeddedResource ContentType = "resource"
	ContentTypeResourceLink     ContentType = "resource_link"
)

// Audio provided to or from an LLM.
type AudioContent struct {
	// The base64-encoded audio data.
	Data string `json:"data" yaml:"data" mapstructure:"data"`

	// The MIME type of the audio.
	MimeType string `json:"mimeType" yaml:"mimeType" mapstructure:"mimeType"`
}

// A link to a resource the server can provide (MCP spec 2025-06-18). The
// resource is not embedded; the client may fetch it via resources/read.
type ResourceLinkContent struct {
	// The URI of the linked resource.
	Uri string `json:"uri" yaml:"uri" mapstructure:"uri"`

	// A programmatic name for the resource.
	Name string `json:"name" yaml:"name" mapstructure:"name"`

	// A human-readable title for the resource.
	Title *string `json:"title,omitempty" yaml:"title,omitempty" mapstructure:"title,omitempty"`

	// A description of what the resource represents.
	Description *string `json:"description,omitempty" yaml:"description,omitempty" mapstructure:"description,omitempty"`

	// The MIME type of the resource, if known.
	MimeType *string `json:"mimeType,omitempty" yaml:"mimeType,omitempty" mapstructure:"mimeType,omitempty"`

	// The size of the raw resource content in bytes, if known.
	Size *int64 `json:"size,omitempty" yaml:"size,omitempty" mapstructure:"size,omitempty"`
}

type Content struct {
	Type                ContentType
	TextContent         *TextContent
	ImageContent        *ImageContent
	AudioContent        *AudioContent
	EmbeddedResource    *EmbeddedResource
	ResourceLinkContent *ResourceLinkContent
	Annotations         *Annotations

	// Raw holds the complete JSON of a content block whose type this library
	// does not know. Such a block is kept instead of failing the whole
	// response; Type then carries the original type string.
	Raw json.RawMessage
}

func (c *Content) UnmarshalJSON(b []byte) error {
	type typeWrapper struct {
		Type             ContentType       `json:"type" yaml:"type" mapstructure:"type"`
		Text             *string           `json:"text" yaml:"text" mapstructure:"text"`
		Data             string            `json:"data" yaml:"data" mapstructure:"data"`
		MimeType         string            `json:"mimeType" yaml:"mimeType" mapstructure:"mimeType"`
		Annotations      *Annotations      `json:"annotations" yaml:"annotations" mapstructure:"annotations"`
		EmbeddedResource *EmbeddedResource `json:"resource" yaml:"resource" mapstructure:"resource"`
	}
	var tw typeWrapper
	err := json.Unmarshal(b, &tw)
	if err != nil {
		return err
	}

	c.Type = tw.Type
	c.Annotations = tw.Annotations

	switch tw.Type {
	case ContentTypeText:
		text := ""
		if tw.Text != nil {
			text = *tw.Text
		}
		c.TextContent = &TextContent{Text: text}
	case ContentTypeImage:
		c.ImageContent = &ImageContent{Data: tw.Data, MimeType: tw.MimeType}
	case ContentTypeAudio:
		c.AudioContent = &AudioContent{Data: tw.Data, MimeType: tw.MimeType}
	case ContentTypeEmbeddedResource:
		c.EmbeddedResource = tw.EmbeddedResource
	case ContentTypeResourceLink:
		var link ResourceLinkContent
		if err := json.Unmarshal(b, &link); err != nil {
			return err
		}
		c.ResourceLinkContent = &link
	default:
		c.Raw = append(json.RawMessage(nil), b...)
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
	case ContentTypeAudio:
		j, err := json.Marshal(c.AudioContent)
		if err != nil {
			return nil, err
		}
		rawJson = j
	case ContentTypeEmbeddedResource:
		// The spec nests the contents under "resource".
		j, err := json.Marshal(struct {
			Resource *EmbeddedResource `json:"resource"`
		}{c.EmbeddedResource})
		if err != nil {
			return nil, err
		}
		rawJson = j
	case ContentTypeResourceLink:
		j, err := json.Marshal(c.ResourceLinkContent)
		if err != nil {
			return nil, err
		}
		rawJson = j
	default:
		if len(c.Raw) > 0 {
			return c.Raw, nil
		}
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

// NewAudioContent creates a new ToolResponse that is audio.
// The given data is base64-encoded
func NewAudioContent(base64EncodedStringData string, mimeType string) *Content {
	return &Content{
		Type:         ContentTypeAudio,
		AudioContent: &AudioContent{Data: base64EncodedStringData, MimeType: mimeType},
	}
}

// NewResourceLinkContent creates a new ToolResponse that links to a resource
// instead of embedding it. Optional fields may be nil.
func NewResourceLinkContent(uri string, name string, title *string, description *string, mimeType *string, size *int64) *Content {
	return &Content{
		Type: ContentTypeResourceLink,
		ResourceLinkContent: &ResourceLinkContent{
			Uri: uri, Name: name, Title: title, Description: description, MimeType: mimeType, Size: size,
		},
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
