package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/metoro-io/mcp-golang/transport"
)

// HTTPTransport implements a stateless HTTP transport for MCP
type HTTPTransport struct {
	server         *http.Server
	endpoint       string
	messageHandler func(message *transport.BaseJsonRpcMessage)
	errorHandler   func(error)
	closeHandler   func()
	mu             sync.RWMutex
	addr           string
}

// NewHTTPTransport creates a new HTTP transport that listens on the specified endpoint
func NewHTTPTransport(endpoint string) *HTTPTransport {
	return &HTTPTransport{
		endpoint: endpoint,
		addr:     ":8080", // Default port
	}
}

// WithAddr sets the address to listen on
func (t *HTTPTransport) WithAddr(addr string) *HTTPTransport {
	t.addr = addr
	return t
}

// Start implements Transport.Start
func (t *HTTPTransport) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc(t.endpoint, t.handleRequest)

	t.server = &http.Server{
		Addr:    t.addr,
		Handler: mux,
	}

	return t.server.ListenAndServe()
}

// Send implements Transport.Send
func (t *HTTPTransport) Send(message *transport.BaseJsonRpcMessage) error {
	return fmt.Errorf("send not supported in stateless HTTP transport - this transport only supports receiving messages")
}

// Close implements Transport.Close
func (t *HTTPTransport) Close() error {
	if t.server != nil {
		if err := t.server.Close(); err != nil {
			return err
		}
	}
	if t.closeHandler != nil {
		t.closeHandler()
	}
	return nil
}

// SetCloseHandler implements Transport.SetCloseHandler
func (t *HTTPTransport) SetCloseHandler(handler func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closeHandler = handler
}

// SetErrorHandler implements Transport.SetErrorHandler
func (t *HTTPTransport) SetErrorHandler(handler func(error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.errorHandler = handler
}

// SetMessageHandler implements Transport.SetMessageHandler
func (t *HTTPTransport) SetMessageHandler(handler func(message *transport.BaseJsonRpcMessage)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.messageHandler = handler
}

func (t *HTTPTransport) handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is supported", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		if t.errorHandler != nil {
			t.errorHandler(fmt.Errorf("failed to read request body: %w", err))
		}
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Try to unmarshal as a request first
	var request transport.BaseJSONRPCRequest
	if err := json.Unmarshal(body, &request); err == nil && request.Id != 0 {
		t.mu.RLock()
		handler := t.messageHandler
		t.mu.RUnlock()

		if handler != nil {
			handler(transport.NewBaseMessageRequest(&request))
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	// Try as a notification
	var notification transport.BaseJSONRPCNotification
	if err := json.Unmarshal(body, &notification); err == nil {
		t.mu.RLock()
		handler := t.messageHandler
		t.mu.RUnlock()

		if handler != nil {
			handler(transport.NewBaseMessageNotification(&notification))
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	// Try as a response
	var response transport.BaseJSONRPCResponse
	if err := json.Unmarshal(body, &response); err == nil {
		t.mu.RLock()
		handler := t.messageHandler
		t.mu.RUnlock()

		if handler != nil {
			handler(transport.NewBaseMessageResponse(&response))
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	// Try as an error
	var errorResponse transport.BaseJSONRPCError
	if err := json.Unmarshal(body, &errorResponse); err == nil {
		t.mu.RLock()
		handler := t.messageHandler
		t.mu.RUnlock()

		if handler != nil {
			handler(transport.NewBaseMessageError(&errorResponse))
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	// If we get here, we couldn't parse the message
	if t.errorHandler != nil {
		t.errorHandler(fmt.Errorf("invalid JSON-RPC message"))
	}
	http.Error(w, "Invalid JSON-RPC message", http.StatusBadRequest)
}
