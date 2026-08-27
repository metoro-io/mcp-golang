package mcp_golang

import (
	"context"
	"encoding/json"

	"github.com/auxdata-ai/mcp-golang/internal1/protocol"
	"github.com/auxdata-ai/mcp-golang/transport"
	"github.com/pkg/errors"
)

// ClientProtocolVersion is the MCP protocol version advertised by the client in the
// initialize request. MCP uses date-based protocol versions; modern servers (e.g.
// GitLab) reject the previously hardcoded "1.0". The server negotiates down to a
// version it supports.
const ClientProtocolVersion = "2025-06-18"

// headerSetter is implemented by transports that carry HTTP headers. Declared
// here instead of referring to the http transport so this package keeps no
// dependency on a concrete transport.
type headerSetter interface {
	SetHeader(key, value string)
}

// Client represents an MCP client that can connect to and interact with MCP servers
type Client struct {
	transport    transport.Transport
	protocol     *protocol.Protocol
	capabilities *ServerCapabilities
	initialized  bool
	info         ClientInfo

	// InitializedNotificationError holds the failure of the initialized
	// notification, if any. Sending it is required by the lifecycle spec but not
	// treated as fatal: servers that answer requests without it worked before it
	// was ever sent, and failing the handshake over it would take them offline.
	// Callers that want the detail can surface this; the next request reports the
	// problem on its own for a server that does insist on the notification.
	InitializedNotificationError error
}

// NewClient creates a new MCP client with the specified transport
func NewClient(transport transport.Transport) *Client {
	return &Client{
		transport: transport,
		protocol:  protocol.NewProtocol(nil),
	}
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// NewClientWithInfo create a new client with info. This is required by anthorpic mcp tools
func NewClientWithInfo(transport transport.Transport, info ClientInfo) *Client {
	return &Client{
		transport: transport,
		protocol:  protocol.NewProtocol(nil),
		info:      info,
	}
}

// Initialize connects to the server and retrieves its capabilities
func (c *Client) Initialize(ctx context.Context) (*InitializeResponse, error) {
	if c.initialized {
		return nil, errors.New("client already initialized")
	}

	err := c.protocol.Connect(c.transport)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect transport")
	}

	// Make initialize request to server
	response, err := c.protocol.Request(ctx, "initialize", map[string]interface{}{
		"protocolVersion": ClientProtocolVersion,
		"capabilities":    map[string]interface{}{},
		"clientInfo":      c.info,
	}, nil)
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

	c.capabilities = &initResult.Capabilities
	c.initialized = true

	// Over HTTP the client MUST send MCP-Protocol-Version on every request after
	// initialize, and it has to be the version negotiated here rather than
	// ClientProtocolVersion: a server MUST answer 400 for a version it does not
	// support, so echoing our wish instead of its answer would break older servers.
	if setter, ok := c.transport.(headerSetter); ok && initResult.ProtocolVersion != "" {
		setter.SetHeader("MCP-Protocol-Version", initResult.ProtocolVersion)
	}

	// The lifecycle spec closes the handshake with this notification; servers
	// SHOULD NOT answer other requests before receiving it.
	c.InitializedNotificationError = c.protocol.Notification("notifications/initialized", map[string]interface{}{})

	return &initResult, nil
}

// paginationParams builds the params object for the paginated list requests.
// The "cursor" key is only set when the caller actually has a cursor from a
// previous page: per the MCP pagination spec a cursor is an opaque token the
// server issued, and servers SHOULD answer -32602 (Invalid params) for a cursor
// they did not hand out. Sending "cursor": "" or "cursor": null on the first
// page therefore breaks strict servers (the Ruby SDK rejects both non-numeric
// strings, the TypeScript SDK rejects null) — the key has to be absent.
func paginationParams(cursor *string) map[string]interface{} {
	params := map[string]interface{}{}

	if cursor != nil {
		params["cursor"] = *cursor
	}

	return params
}

// ListTools retrieves the list of available tools from the server
func (c *Client) ListTools(ctx context.Context, cursor *string) (*ToolsResponse, error) {
	if !c.initialized {
		return nil, errors.New("client not initialized")
	}

	params := paginationParams(cursor)

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

	params := paginationParams(cursor)

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

	params := paginationParams(cursor)

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
