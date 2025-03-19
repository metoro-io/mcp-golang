package mcp_golang

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/rvoh-emccaleb/mcp-golang/internal/protocol"
	"github.com/rvoh-emccaleb/mcp-golang/transport"
)

// Client represents an MCP client that can connect to and interact with MCP servers
type Client struct {
	transport    transport.Transport
	protocol     *protocol.Protocol
	capabilities *ServerCapabilities
	initialized  bool
}

// NewClient creates a new MCP client with the specified transport
func NewClient(transport transport.Transport) *Client {
	return &Client{
		transport: transport,
		protocol:  protocol.NewProtocol(nil),
	}
}

// A bit loosey goosey, but it works for now.
// See: https://spec.modelcontextprotocol.io/specification/2024-11-05/basic/lifecycle/
type InitializeRequestParams struct {
	ProtocolVersion string                      `json:"protocolVersion"`
	ClientInfo      InitializeRequestClientInfo `json:"clientInfo"`
	Capabilities    map[string]interface{}      `json:"capabilities"`
}

type InitializeRequestClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Initialize connects to the server and retrieves its capabilities
func (c *Client) Initialize(ctx context.Context, params *InitializeRequestParams) (*InitializeResponse, error) {
	if c.initialized {
		return nil, errors.New("client already initialized")
	}

	err := c.protocol.Connect(c.transport)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect transport")
	}

	// Begin initialization handshake with server
	response, err := c.protocol.Request(ctx, "initialize", params, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize")
	}

	responseBytes, ok := response.(json.RawMessage)
	if !ok {
		return nil, errors.New("invalid response type")
	}

	var initResult InitializeResponse
	err = json.Unmarshal(responseBytes, &initResult)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal initialize response")
	}

	// Finish initialization handshake with server
	err = c.protocol.Notification("notifications/initialized", map[string]interface{}{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to send initialized notification")
	}

	c.capabilities = &initResult.Capabilities
	c.initialized = true

	return &initResult, nil
}

// ListTools retrieves the list of available tools from the server
func (c *Client) ListTools(ctx context.Context, cursor *string) (*ToolsResponse, error) {
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

	responseBytes, ok := response.(json.RawMessage)
	if !ok {
		return nil, errors.New("invalid response type")
	}

	var toolsResponse ToolsResponse
	err = json.Unmarshal(responseBytes, &toolsResponse)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal tools response")
	}

	return &toolsResponse, nil
}

// CallTool calls a specific tool on the server with the provided arguments
func (c *Client) CallTool(ctx context.Context, name string, arguments any) (*ToolResponse, error) {
	if !c.initialized {
		return nil, errors.New("client not initialized")
	}

	argumentsJson, err := json.Marshal(arguments)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal arguments")
	}

	params := baseCallToolRequestParams{
		Name:      name,
		Arguments: argumentsJson,
	}

	response, err := c.protocol.Request(ctx, "tools/call", params, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call tool")
	}

	responseBytes, ok := response.(json.RawMessage)
	if !ok {
		return nil, errors.New("invalid response type")
	}

	var toolResponse ToolResponse
	err = json.Unmarshal(responseBytes, &toolResponse)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal tool response")
	}

	return &toolResponse, nil
}

// ListPrompts retrieves the list of available prompts from the server
func (c *Client) ListPrompts(ctx context.Context, cursor *string) (*ListPromptsResponse, error) {
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

	responseBytes, ok := response.(json.RawMessage)
	if !ok {
		return nil, errors.New("invalid response type")
	}

	var promptsResponse ListPromptsResponse
	err = json.Unmarshal(responseBytes, &promptsResponse)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal prompts response")
	}

	return &promptsResponse, nil
}

// GetPrompt retrieves a specific prompt from the server
func (c *Client) GetPrompt(ctx context.Context, name string, arguments any) (*PromptResponse, error) {
	if !c.initialized {
		return nil, errors.New("client not initialized")
	}

	argumentsJson, err := json.Marshal(arguments)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal arguments")
	}

	params := baseGetPromptRequestParamsArguments{
		Name:      name,
		Arguments: argumentsJson,
	}

	response, err := c.protocol.Request(ctx, "prompts/get", params, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get prompt")
	}

	responseBytes, ok := response.(json.RawMessage)
	if !ok {
		return nil, errors.New("invalid response type")
	}

	var promptResponse PromptResponse
	err = json.Unmarshal(responseBytes, &promptResponse)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal prompt response")
	}

	return &promptResponse, nil
}

// ListResources retrieves the list of available resources from the server
func (c *Client) ListResources(ctx context.Context, cursor *string) (*ListResourcesResponse, error) {
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

	responseBytes, ok := response.(json.RawMessage)
	if !ok {
		return nil, errors.New("invalid response type")
	}

	var resourcesResponse ListResourcesResponse
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

	responseBytes, ok := response.(json.RawMessage)
	if !ok {
		return nil, errors.New("invalid response type")
	}

	var resourceResponse ResourceResponse
	err = json.Unmarshal(responseBytes, &resourceResponse)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal resource response")
	}

	return &resourceResponse, nil
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
func (c *Client) GetCapabilities() *ServerCapabilities {
	return c.capabilities
}

// Close cleans up resources used by the client, including the protocol and transport layers.
// It should be called when the client is no longer needed.
func (c *Client) Close() error {
	if err := c.protocol.Close(); err != nil {
		return errors.Wrap(err, "failed to close protocol")
	}

	return nil
}
