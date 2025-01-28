package mcp_golang

type readResourceRequestParams struct {
	// The URI of the resource to read. The URI can use any protocol; it is up to the
	// server how to interpret it.
	Uri string `json:"uri" yaml:"uri" mapstructure:"uri"`
}

// The server's response to a resources/list request from the client.
type ListResourcesResponse struct {
	// Resources corresponds to the JSON schema field "resources".
	Resources []*ResourceSchema `json:"resources" yaml:"resources" mapstructure:"resources"`
	// NextCursor is a cursor for pagination. If not nil, there are more resources available.
	NextCursor *string `json:"nextCursor,omitempty" yaml:"nextCursor,omitempty" mapstructure:"nextCursor,omitempty"`
}

// A known resource that the server is capable of reading.
type ResourceSchema struct {
	// Annotations corresponds to the JSON schema field "annotations".
	Annotations *Annotations `json:"annotations,omitempty" yaml:"annotations,omitempty" mapstructure:"annotations,omitempty"`

	// A description of what this resource represents.
	//
	// This can be used by clients to improve the LLM's understanding of available
	// resources. It can be thought of like a "hint" to the model.
	Description *string `json:"description,omitempty" yaml:"description,omitempty" mapstructure:"description,omitempty"`

	// The MIME type of this resource, if known.
	MimeType *string `json:"mimeType,omitempty" yaml:"mimeType,omitempty" mapstructure:"mimeType,omitempty"`

	// A human-readable name for this resource.
	//
	// This can be used by clients to populate UI elements.
	Name string `json:"name" yaml:"name" mapstructure:"name"`

	// The URI of this resource.
	Uri string `json:"uri" yaml:"uri" mapstructure:"uri"`
}

// A resource template that defines a pattern for dynamic resources.
type ResourceTemplateSchema struct {
	// Annotations corresponds to the JSON schema field "annotations".
	Annotations *Annotations `json:"annotations,omitempty" yaml:"annotations,omitempty" mapstructure:"annotations,omitempty"`

	// A description of what resources matching this template represent.
	Description *string `json:"description,omitempty" yaml:"description,omitempty" mapstructure:"description,omitempty"`

	// The MIME type of resources matching this template, if known.
	MimeType *string `json:"mimeType,omitempty" yaml:"mimeType,omitempty" mapstructure:"mimeType,omitempty"`

	// A human-readable name for this template.
	Name string `json:"name" yaml:"name" mapstructure:"name"`

	// The URI template following RFC 6570.
	UriTemplate string `json:"uriTemplate" yaml:"uriTemplate" mapstructure:"uriTemplate"`
}

// The server's response to a resources/templates/list request from the client.
type ListResourceTemplatesResponse struct {
	// Templates corresponds to the JSON schema field "templates".
	Templates []*ResourceTemplateSchema `json:"resourceTemplates" yaml:"resourceTemplates" mapstructure:"resourceTemplates"`
	// NextCursor is a cursor for pagination. If not nil, there are more templates available.
	NextCursor *string `json:"nextCursor,omitempty" yaml:"nextCursor,omitempty" mapstructure:"nextCursor,omitempty"`
}
