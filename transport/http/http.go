package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/metoro-io/mcp-golang/transport"
)

// HTTPTransport implements a stateless HTTP transport for MCP
type HTTPTransport struct {
	*baseTransport
	server         *http.Server
	endpoint       string
	messageHandler func(ctx context.Context, message *transport.BaseJsonRpcMessage)
	errorHandler   func(error)
	closeHandler   func()
	mu             sync.RWMutex
	addr           string
}

// NewHTTPTransport creates a new HTTP transport that listens on the specified endpoint
func NewHTTPTransport(endpoint string) *HTTPTransport {
	return &HTTPTransport{
		baseTransport: newBaseTransport(),
		endpoint:      endpoint,
		addr:          ":8080", // Default port
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
func (t *HTTPTransport) Send(ctx context.Context, message *transport.BaseJsonRpcMessage) error {
	key := message.JsonRpcResponse.Id
	fmt.Printf("[Send] Attempting to send response with key: %d\n", key)

	responseChannel := t.baseTransport.responseMap[int64(key)]
	if responseChannel == nil {
		fmt.Printf("[Send] Response map keys: %v\n", t.getResponseMapKeys())

		return fmt.Errorf("no response channel found for key: %d", key)
	}
	responseChannel <- message
	return nil
}

// Helper method to get keys
func (t *HTTPTransport) getResponseMapKeys() []int64 {
	keys := make([]int64, 0, len(t.baseTransport.responseMap))
	for k := range t.baseTransport.responseMap {
		keys = append(keys, k)
	}
	return keys
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
func (t *HTTPTransport) SetMessageHandler(handler func(ctx context.Context, message *transport.BaseJsonRpcMessage)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.baseTransport.messageHandler = handler
	t.messageHandler = handler
}

func (t *HTTPTransport) handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is supported", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	body, err := t.readBody(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response, err := t.handleMessage(ctx, body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		if t.errorHandler != nil {
			t.errorHandler(fmt.Errorf("failed to marshal response: %w", err))
		}
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}
