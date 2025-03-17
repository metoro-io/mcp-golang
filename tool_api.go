package mcp_golang

// This is a union type of all the different ToolResponse that can be sent back to the client.
// We allow creation through constructors only to make sure that the ToolResponse is valid.
type ToolResponse struct {
	Status  int        `json:"status" yaml:"status" mapstructure:"status"`
	Content []*Content `json:"content" yaml:"content" mapstructure:"content"`
}

func NewToolResponse(content ...*Content) *ToolResponse {
	return &ToolResponse{
		Content: content,
	}
}

func NewToolResponseWithStatus(status int, content ...*Content) *ToolResponse {
	return &ToolResponse{
		Status:  status,
		Content: content,
	}
}
