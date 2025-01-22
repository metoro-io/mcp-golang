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
	responseMap    map[int64]chan *transport.BaseJsonRpcMessage
}

// NewHTTPTransport creates a new HTTP transport that listens on the specified endpoint
func NewHTTPTransport(endpoint string) *HTTPTransport {
	return &HTTPTransport{
		endpoint:    endpoint,
		addr:        ":8080", // Default port
		responseMap: make(map[int64]chan *transport.BaseJsonRpcMessage),
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
	key := message.JsonRpcResponse.Id
	responseChannel := t.responseMap[int64(key)]
	if responseChannel == nil {
		return fmt.Errorf("no response channel found for key: %d", key)
	}
	responseChannel <- message
	return nil
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

	// Store the response writer for later use
	t.mu.Lock()
	var key int64 = 0

	for key < 1000000 {
		if _, ok := t.responseMap[key]; !ok {
			break
		}
		key = key + 1
	}
	t.responseMap[key] = make(chan *transport.BaseJsonRpcMessage)
	t.mu.Unlock()

	var prevId *transport.RequestId = nil
	deserialized := false
	// Try to unmarshal as a request first
	var request transport.BaseJSONRPCRequest
	if err := json.Unmarshal(body, &request); err == nil {
		deserialized = true
		id := request.Id
		prevId = &id
		request.Id = transport.RequestId(key)
		t.mu.RLock()
		handler := t.messageHandler
		t.mu.RUnlock()

		if handler != nil {
			handler(transport.NewBaseMessageRequest(&request))
		}
	}

	// Try as a notification
	var notification transport.BaseJSONRPCNotification
	if !deserialized {
		if err := json.Unmarshal(body, &notification); err == nil {
			deserialized = true
			t.mu.RLock()
			handler := t.messageHandler
			t.mu.RUnlock()

			if handler != nil {
				handler(transport.NewBaseMessageNotification(&notification))
			}
		}
	}

	// Try as a response
	var response transport.BaseJSONRPCResponse
	if !deserialized {
		if err := json.Unmarshal(body, &response); err == nil {
			deserialized = true
			t.mu.RLock()
			handler := t.messageHandler
			t.mu.RUnlock()

			if handler != nil {
				handler(transport.NewBaseMessageResponse(&response))
			}
		}
	}

	// Try as an error
	var errorResponse transport.BaseJSONRPCError
	if !deserialized {
		if err := json.Unmarshal(body, &errorResponse); err == nil {
			deserialized = true
			t.mu.RLock()
			handler := t.messageHandler
			t.mu.RUnlock()

			if handler != nil {
				handler(transport.NewBaseMessageError(&errorResponse))
			}
		}
	}

	// Block until the response is received
	responseToUse := <-t.responseMap[key]
	delete(t.responseMap, key)
	if prevId != nil {
		responseToUse.JsonRpcResponse.Id = *prevId
	}
	jsonData, err := json.Marshal(responseToUse)
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
