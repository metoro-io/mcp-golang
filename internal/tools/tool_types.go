package tools

// Definition for a tool the client can call.
type ToolRetType struct {
	// A human-readable description of the tool.
	Description *string `json:"description,omitempty" yaml:"description,omitempty" mapstructure:"description,omitempty"`

	// A JSON Schema object defining the expected parameters for the tool.
	InputSchema interface{} `json:"inputSchema" yaml:"inputSchema" mapstructure:"inputSchema"`

	// The name of the tool.
	Name string `json:"name" yaml:"name" mapstructure:"name"`
}
type ToolsResponse struct {
	Tools      []ToolRetType `json:"tools" yaml:"tools" mapstructure:"tools"`
	NextCursor *string       `json:"nextCursor,omitempty" yaml:"nextCursor,omitempty" mapstructure:"nextCursor,omitempty"`
}
