package mcp_golang

import (
	"context"
	"encoding/json"

	"github.com/metoro-io/mcp-golang/internal/protocol"
	"github.com/metoro-io/mcp-golang/internal/tools"
	"github.com/metoro-io/mcp-golang/transport"
	"github.com/pkg/errors"
)

// Client represents an MCP client that can connect to and interact with MCP servers
type Client struct {
	transport    transport.Transport
	protocol     *protocol.Protocol
	capabilities *serverCapabilities
	initialized  bool
}

// NewClient creates a new MCP client with the specified transport
func NewClient(transport transport.Transport) *Client {
	return &Client{
		transport: transport,
		protocol:  protocol.NewProtocol(nil),
	}
}

// Initialize connects to the server and retrieves its capabilities
func (c *Client) Initialize(ctx context.Context) error {
	if c.initialized {
		return errors.New("client already initialized")
	}

	err := c.protocol.Connect(c.transport)
	if err != nil {
		return errors.Wrap(err, "failed to connect transport")
	}

	// Make initialize request to server
	response, err := c.protocol.Request(ctx, "initialize", nil, nil)
	if err != nil {
		return errors.Wrap(err, "failed to initialize")
	}

	responseBytes, ok := response.([]byte)
	if !ok {
		return errors.New("invalid response type")
	}

	var initResult initializeResult
	err = json.Unmarshal(responseBytes, &initResult)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal initialize response")
	}

	c.capabilities = &initResult.Capabilities
	c.initialized = true
	return nil
}

// ListTools retrieves the list of available tools from the server
func (c *Client) ListTools(ctx context.Context, cursor *string) (*tools.ToolsResponse, error) {
	if !c.initialized {
		return nil, errors.New("client not initialized")
	}

	params := map[string]interface{}{
		"cursor": cursor,
	}

	response, err := c.protocol.Request(ctx, "tools/list", params, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list tools")
	}

	responseBytes, ok := response.([]byte)
	if !ok {
		return nil, errors.New("invalid response type")
	}

	var toolsResponse tools.ToolsResponse
	err = json.Unmarshal(responseBytes, &toolsResponse)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal tools response")
	}

	return &toolsResponse, nil
}

// CallTool calls a specific tool on the server with the provided arguments
func (c *Client) CallTool(ctx context.Context, name string, arguments json.RawMessage) (*ToolResponse, error) {
	if !c.initialized {
		return nil, errors.New("client not initialized")
	}

	params := baseCallToolRequestParams{
		Name:      name,
		Arguments: arguments,
	}

	response, err := c.protocol.Request(ctx, "tools/call", params, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call tool")
	}

	responseBytes, ok := response.([]byte)
	if !ok {
		return nil, errors.New("invalid response type")
	}

	var toolResponse toolResponseSent
	err = json.Unmarshal(responseBytes, &toolResponse)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal tool response")
	}

	if toolResponse.Error != nil {
		return nil, toolResponse.Error
	}

	return toolResponse.Response, nil
}

// ListPrompts retrieves the list of available prompts from the server
func (c *Client) ListPrompts(ctx context.Context, cursor *string) (*listPromptsResult, error) {
	if !c.initialized {
		return nil, errors.New("client not initialized")
	}

	params := map[string]interface{}{
		"cursor": cursor,
	}

	response, err := c.protocol.Request(ctx, "prompts/list", params, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list prompts")
	}

	responseBytes, ok := response.([]byte)
	if !ok {
		return nil, errors.New("invalid response type")
	}

	var promptsResponse listPromptsResult
	err = json.Unmarshal(responseBytes, &promptsResponse)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal prompts response")
	}

	return &promptsResponse, nil
}

// GetPrompt retrieves a specific prompt from the server
func (c *Client) GetPrompt(ctx context.Context, name string, arguments json.RawMessage) (*PromptResponse, error) {
	if !c.initialized {
		return nil, errors.New("client not initialized")
	}

	params := baseGetPromptRequestParamsArguments{
		Name:      name,
		Arguments: arguments,
	}

	response, err := c.protocol.Request(ctx, "prompts/get", params, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get prompt")
	}

	responseBytes, ok := response.([]byte)
	if !ok {
		return nil, errors.New("invalid response type")
	}

	var promptResponse promptResponseSent
	err = json.Unmarshal(responseBytes, &promptResponse)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal prompt response")
	}

	if promptResponse.Error != nil {
		return nil, promptResponse.Error
	}

	return promptResponse.Response, nil
}

// ListResources retrieves the list of available resources from the server
func (c *Client) ListResources(ctx context.Context, cursor *string) (*listResourcesResult, error) {
	if !c.initialized {
		return nil, errors.New("client not initialized")
	}

	params := map[string]interface{}{
		"cursor": cursor,
	}

	response, err := c.protocol.Request(ctx, "resources/list", params, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list resources")
	}

	responseBytes, ok := response.([]byte)
	if !ok {
		return nil, errors.New("invalid response type")
	}

	var resourcesResponse listResourcesResult
	err = json.Unmarshal(responseBytes, &resourcesResponse)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal resources response")
	}

	return &resourcesResponse, nil
}

// ReadResource reads a specific resource from the server
func (c *Client) ReadResource(ctx context.Context, uri string) (*ResourceResponse, error) {
	if !c.initialized {
		return nil, errors.New("client not initialized")
	}

	params := readResourceRequestParams{
		Uri: uri,
	}

	response, err := c.protocol.Request(ctx, "resources/read", params, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read resource")
	}

	responseBytes, ok := response.([]byte)
	if !ok {
		return nil, errors.New("invalid response type")
	}

	var resourceResponse resourceResponseSent
	err = json.Unmarshal(responseBytes, &resourceResponse)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal resource response")
	}

	if resourceResponse.Error != nil {
		return nil, resourceResponse.Error
	}

	return resourceResponse.Response, nil
}

// Ping sends a ping request to the server to check connectivity
func (c *Client) Ping(ctx context.Context) error {
	if !c.initialized {
		return errors.New("client not initialized")
	}

	_, err := c.protocol.Request(ctx, "ping", nil, nil)
	if err != nil {
		return errors.Wrap(err, "failed to ping server")
	}

	return nil
}

// GetCapabilities returns the server capabilities obtained during initialization
func (c *Client) GetCapabilities() *serverCapabilities {
	return c.capabilities
}
