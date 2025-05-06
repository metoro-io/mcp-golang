package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/metoro-io/mcp-golang/transport"
)

// HTTPClientTransport implements a client-side HTTP transport for MCP
type HTTPClientTransport struct {
	baseURL        string
	endpoint       string
	messageHandler func(ctx context.Context, message *transport.BaseJsonRpcMessage)
	errorHandler   func(error)
	closeHandler   func()
	mu             sync.RWMutex
	client         *http.Client
	headers        map[string]string
}

// NewHTTPClientTransport creates a new HTTP client transport that connects to the specified endpoint
func NewHTTPClientTransport(endpoint string) *HTTPClientTransport {
	return &HTTPClientTransport{
		endpoint: endpoint,
		client:   &http.Client{},
		headers:  make(map[string]string),
	}
}

// WithBaseURL sets the base URL to connect to
func (t *HTTPClientTransport) WithBaseURL(baseURL string) *HTTPClientTransport {
	t.baseURL = baseURL
	return t
}

// WithHeader adds a header to the request
func (t *HTTPClientTransport) WithHeader(key, value string) *HTTPClientTransport {
	t.headers[key] = value
	return t
}

// WithClient sets the HTTP client to use for requests
func (t *HTTPClientTransport) WithClient(client *http.Client) *HTTPClientTransport {
	t.client = client
	return t
}

// Start implements Transport.Start
func (t *HTTPClientTransport) Start(ctx context.Context) error {
	// Does nothing in the stateless http client transport
	return nil
}

// Send implements Transport.Send
func (t *HTTPClientTransport) Send(ctx context.Context, message *transport.BaseJsonRpcMessage) error {
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	url := fmt.Sprintf("%s%s", t.baseURL, t.endpoint)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned error: %s (status: %d)", string(body), resp.StatusCode)
	}

	if len(body) > 0 {
		// Try to unmarshal as a response first
		var response transport.BaseJSONRPCResponse
		if err := json.Unmarshal(body, &response); err == nil {
			t.mu.RLock()
			handler := t.messageHandler
			t.mu.RUnlock()

			if handler != nil {
				handler(ctx, transport.NewBaseMessageResponse(&response))
			}
			return nil
		}

		// Try as an error
		var errorResponse transport.BaseJSONRPCError
		if err := json.Unmarshal(body, &errorResponse); err == nil {
			t.mu.RLock()
			handler := t.messageHandler
			t.mu.RUnlock()

			if handler != nil {
				handler(ctx, transport.NewBaseMessageError(&errorResponse))
			}
			return nil
		}

		// Try as a notification
		var notification transport.BaseJSONRPCNotification
		if err := json.Unmarshal(body, &notification); err == nil {
			t.mu.RLock()
			handler := t.messageHandler
			t.mu.RUnlock()

			if handler != nil {
				handler(ctx, transport.NewBaseMessageNotification(&notification))
			}
			return nil
		}

		// Try as a request
		var request transport.BaseJSONRPCRequest
		if err := json.Unmarshal(body, &request); err == nil {
			t.mu.RLock()
			handler := t.messageHandler
			t.mu.RUnlock()

			if handler != nil {
				handler(ctx, transport.NewBaseMessageRequest(&request))
			}
			return nil
		}

		return fmt.Errorf("received invalid response: %s", string(body))
	}

	return nil
}

// Close implements Transport.Close
func (t *HTTPClientTransport) Close() error {
	if t.closeHandler != nil {
		t.closeHandler()
	}
	return nil
}

// SetCloseHandler implements Transport.SetCloseHandler
func (t *HTTPClientTransport) SetCloseHandler(handler func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closeHandler = handler
}

// SetErrorHandler implements Transport.SetErrorHandler
func (t *HTTPClientTransport) SetErrorHandler(handler func(error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.errorHandler = handler
}

// SetMessageHandler implements Transport.SetMessageHandler
func (t *HTTPClientTransport) SetMessageHandler(handler func(ctx context.Context, message *transport.BaseJsonRpcMessage)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.messageHandler = handler
}
