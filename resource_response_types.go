package mcp_golang

type readResourceRequestParams struct {
	// The URI of the resource to read. The URI can use any protocol; it is up to the
	// server how to interpret it.
	Uri string `json:"uri" yaml:"uri" mapstructure:"uri"`
}

// The server's response to a resources/list request from the client.
type listResourcesResult struct {
	// Resources corresponds to the JSON schema field "resources".
	Resources []*resourceSchema `json:"resources" yaml:"resources" mapstructure:"resources"`
}

// A known resource that the server is capable of reading.
type resourceSchema struct {
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
