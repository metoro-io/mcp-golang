package mcp_golang

type PromptMessage struct {
	Content *Content `json:"content" yaml:"content" mapstructure:"content"`
	Role    Role     `json:"role" yaml:"role" mapstructure:"role"`
}

func NewPromptMessage(content *Content, role Role) *PromptMessage {
	return &PromptMessage{
		Content: content,
		Role:    role,
	}
}

// The server's response to a prompts/get request from the client.
type PromptResponse struct {
	// An optional description for the prompt.
	Description *string `json:"description,omitempty" yaml:"description,omitempty" mapstructure:"description,omitempty"`

	// Messages corresponds to the JSON schema field "messages".
	Messages []*PromptMessage `json:"messages" yaml:"messages" mapstructure:"messages"`
}

func NewPromptResponse(description string, messages ...*PromptMessage) *PromptResponse {
	return &PromptResponse{
		Description: &description,
		Messages:    messages,
	}
}
