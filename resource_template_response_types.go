package mcp_golang

// The server's response to a resources/templates/list request from the client.
type ListResourceTemplatesResponse struct {
	// Templates corresponds to the JSON schema field "templates".
	Templates []*ResourceTemplateSchema `json:"templates" yaml:"templates" mapstructure:"templates"`
	// NextCursor is a cursor for pagination. If not nil, there are more templates available.
	NextCursor *string `json:"nextCursor,omitempty" yaml:"nextCursor,omitempty" mapstructure:"nextCursor,omitempty"`
}

// A resource template that the server is capable of instantiating.
type ResourceTemplateSchema struct {
	// Annotations corresponds to the JSON schema field "annotations".
	Annotations *Annotations `json:"annotations,omitempty" yaml:"annotations,omitempty" mapstructure:"annotations,omitempty"`

	// A description of what this resource template represents.
	//
	// This can be used by clients to improve the LLM's understanding of available
	// resource templates. It can be thought of like a "hint" to the model.
	Description *string `json:"description,omitempty" yaml:"description,omitempty" mapstructure:"description,omitempty"`

	// The MIME type of resources created from this template.
	MimeType *string `json:"mimeType,omitempty" yaml:"mimeType,omitempty" mapstructure:"mimeType,omitempty"`

	// A human-readable name for this resource template.
	//
	// This can be used by clients to populate UI elements.
	Name string `json:"name" yaml:"name" mapstructure:"name"`

	// The URI of this resource template.
	Uri string `json:"uri" yaml:"uri" mapstructure:"uri"`

	// Parameters that can be used to instantiate this template.
	Parameters []ResourceTemplateParameter `json:"parameters" yaml:"parameters" mapstructure:"parameters"`
}

// A parameter that can be used to instantiate a resource template.
type ResourceTemplateParameter struct {
	// A description of what this parameter represents.
	//
	// This can be used by clients to improve the LLM's understanding of the parameter.
	Description *string `json:"description,omitempty" yaml:"description,omitempty" mapstructure:"description,omitempty"`

	// A human-readable name for this parameter.
	//
	// This can be used by clients to populate UI elements.
	Name string `json:"name" yaml:"name" mapstructure:"name"`

	// Whether this parameter is required when instantiating the template.
	Required *bool `json:"required,omitempty" yaml:"required,omitempty" mapstructure:"required,omitempty"`
}
