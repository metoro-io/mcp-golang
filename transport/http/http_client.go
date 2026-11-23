package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/auxdata-ai/mcp-golang/transport"
)

type HTTPClient interface {
	Do(r *http.Request) (*http.Response, error)
}

// HTTPClientTransport implements a client-side HTTP transport for MCP
type HTTPClientTransport struct {
	baseURL        string
	endpoint       string
	messageHandler func(ctx context.Context, message *transport.BaseJsonRpcMessage)
	errorHandler   func(error)
	closeHandler   func()
	mu             sync.RWMutex
	client         HTTPClient
	headers        map[string]string
	McpSessionId   string
}

// NewHTTPClientTransport creates a new HTTP client transport that connects to the specified endpoint
func NewHTTPClientTransport(endpoint string) *HTTPClientTransport {
	return &HTTPClientTransport{
		endpoint: endpoint,
		client:   &http.Client{},
		headers:  make(map[string]string),
	}
}

// WithClient allows to set a custom HTTP client
func (t *HTTPClientTransport) WithClient(c HTTPClient) *HTTPClientTransport {
	t.client = c
	return t
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

// SetHeader sets a header for all subsequent requests. Unlike WithHeader it does
// not chain, so callers that cannot import this package can reach it through a
// narrow interface (see the protocol version header set after initialize).
func (t *HTTPClientTransport) SetHeader(key, value string) {
	t.headers[key] = value
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

	if t.McpSessionId != "" {
		req.Header.Set("Mcp-Session-Id", t.McpSessionId)
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

	// Any 2xx is a success. A JSON-RPC notification or response has no reply, and
	// the spec has the server acknowledge those with "202 Accepted with no body" —
	// insisting on exactly 200 turned that correct answer into a transport error.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server returned error: %s (status: %d)", string(body), resp.StatusCode)
	}

	if t.McpSessionId == "" {
		t.McpSessionId = resp.Header.Get("Mcp-Session-Id")
	}

	// Trimming first so an acknowledgement padded with a newline still counts as
	// the empty body it is meant to be, instead of failing the JSON extraction.
	body = bytes.TrimSpace(body)

	if len(body) > 0 {
		// Try to unmarshal as a response first

		// find start index of "{"
		start := strings.Index(string(body), "{")

		// find last index of "}"
		end := strings.LastIndex(string(body), "}")

		if start != -1 && end != -1 {
			// Erstellung des neuen Strings
			body = body[start : end+1]
		} else {
			return fmt.Errorf("no json found in response. response body: %s", body)
		}

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
