package sse

import (
	"context"
	"fmt"
	sse2 "github.com/metoro-io/mcp-golang/transport/sse/internal/sse"
	"io"
	"net/http"
)

// SSEServerTransport implements a server-side SSE transport
type SSEServerTransport struct {
	transport *sse2.SSETransport
}

// NewSSEServerTransport creates a new SSE server transport
func NewSSEServerTransport(endpoint string, w http.ResponseWriter) (*SSEServerTransport, error) {
	transport, err := sse2.NewSSETransport(endpoint, w)
	if err != nil {
		return nil, err
	}

	return &SSEServerTransport{
		transport: transport,
	}, nil
}

// Start initializes the SSE connection
func (s *SSEServerTransport) Start(ctx context.Context) error {
	return s.transport.Start(ctx)
}

// HandlePostMessage processes an incoming POST request containing a JSON-RPC message
func (s *SSEServerTransport) HandlePostMessage(r *http.Request) error {
	if r.Method != http.MethodPost {
		return fmt.Errorf("method not allowed: %s", r.Method)
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		return fmt.Errorf("unsupported Content type: %s", contentType)
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, sse2.maxMessageSize))
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}
	defer r.Body.Close()

	return s.transport.HandleMessage(body)
}

// Send sends a message over the SSE connection
func (s *SSEServerTransport) Send(msg JSONRPCMessage) error {
	return s.transport.Send(msg)
}

// Close closes the SSE connection
func (s *SSEServerTransport) Close() error {
	return s.transport.Close()
}

// SetCloseHandler sets the callback for when the connection is closed
func (s *SSEServerTransport) SetCloseHandler(handler func()) {
	s.transport.SetCloseHandler(handler)
}

// SetErrorHandler sets the callback for when an error occurs
func (s *SSEServerTransport) SetErrorHandler(handler func(error)) {
	s.transport.SetErrorHandler(handler)
}

// SetMessageHandler sets the callback for when a message is received
func (s *SSEServerTransport) SetMessageHandler(handler func(JSONRPCMessage)) {
	s.transport.SetMessageHandler(handler)
}

// SessionID returns the unique session identifier for this transport
func (s *SSEServerTransport) SessionID() string {
	return s.transport.SessionID()
}
