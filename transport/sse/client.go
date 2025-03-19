package sse

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rvoh-emccaleb/mcp-golang/transport"
)

// SSEClientTransport implements a client-side SSE transport for MCP
type SSEClientTransport struct {
	baseURL            string
	endpoint           string
	postEndpoint       string
	messageHandler     func(ctx context.Context, message *transport.BaseJsonRpcMessage)
	errorHandler       func(error)
	closeHandler       func()
	mu                 sync.RWMutex
	client             *http.Client
	notificationClient *http.Client
	headers            map[string]string
	done               chan struct{}
}

// NewSSEClientTransport creates a new SSE client transport that connects to the specified endpoint
func NewSSEClientTransport(endpoint string, notificationTimeout time.Duration) *SSEClientTransport {
	if notificationTimeout <= 0 {
		notificationTimeout = 1 * time.Millisecond // This is flaky, but it works for local demos.
	}

	return &SSEClientTransport{
		endpoint: endpoint,
		client:   &http.Client{},
		notificationClient: &http.Client{
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
			Timeout: notificationTimeout,
		},
		headers: make(map[string]string),
		done:    make(chan struct{}),
	}
}

// WithBaseURL sets the base URL to connect to
func (t *SSEClientTransport) WithBaseURL(baseURL string) *SSEClientTransport {
	t.baseURL = baseURL
	return t
}

// WithHeader adds a header to the request
func (t *SSEClientTransport) WithHeader(key, value string) *SSEClientTransport {
	t.headers[key] = value
	return t
}

// Start implements Transport.Start
func (t *SSEClientTransport) Start(ctx context.Context) error {
	url, err := url.JoinPath(t.baseURL, t.endpoint)
	if err != nil {
		return fmt.Errorf("failed to construct URL: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set required SSE headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}

	endpointChan := make(chan struct{}, 1)

	// Establish our SSE connection and start reading SSE messages in the background
	go func() {
		// Response should return once we receive headers (body can have data written to it over time).
		resp, err := t.client.Do(req)
		if err != nil {
			if t.errorHandler != nil {
				t.errorHandler(fmt.Errorf("failed to establish SSE connection: %w", err))
			}
			return
		}

		// Validate SSE connection
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			if t.errorHandler != nil {
				t.errorHandler(fmt.Errorf("server returned error status %d: %s", resp.StatusCode, string(body)))
			}
			resp.Body.Close()
			return
		}

		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/event-stream") {
			if t.errorHandler != nil {
				t.errorHandler(fmt.Errorf("invalid content type for SSE connection: %s", contentType))
			}
			resp.Body.Close()
			return
		}

		// Start reading and storing SSE messages
		t.readSSEMessages(resp.Body, endpointChan)
	}()

	// Wait for either the endpoint event or timeout
	select {
	case <-endpointChan:
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for endpoint event")
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting for endpoint event: %w", ctx.Err())
	}
}

// readSSEMessages reads SSE messages from the response body and processes them
func (t *SSEClientTransport) readSSEMessages(body io.ReadCloser, endpointChan chan<- struct{}) {
	defer body.Close()

	reader := bufio.NewReader(body)
	buffer := make([]byte, 4096)
	var messageBuffer bytes.Buffer

	for {
		select {
		case <-t.done:
			return
		default:
			n, err := reader.Read(buffer)
			if err != nil {
				if err != io.EOF {
					if t.errorHandler != nil {
						t.errorHandler(fmt.Errorf("error reading SSE stream: %w", err))
					}
				}
				return
			}

			messageBuffer.Write(buffer[:n])

			// Process complete messages (terminated by double newline)
			for {
				// Check if we have at least one complete message (terminated by double newline)
				content := messageBuffer.String()
				messageEnd := strings.Index(content, "\n\n")
				if messageEnd == -1 {
					// No complete message yet, keep reading
					break
				}

				message := content[:messageEnd]

				messageBuffer.Reset()
				messageBuffer.WriteString(content[messageEnd+2:]) // 2 newlines

				// Parse the complete message
				msg, err := t.parseSSEMessageFromString(message, endpointChan)
				if err != nil {
					if t.errorHandler != nil {
						t.errorHandler(fmt.Errorf("error parsing SSE message: %w", err))
					}
					continue
				}

				// Handle the message based on its type.
				// Note: The initial endpoint event doesn't require handling, here.
				//       We only need to handle the response and notification types.
				switch msg.Type {
				case transport.BaseMessageTypeJSONRPCResponseType,
					transport.BaseMessageTypeJSONRPCNotificationType,
					transport.BaseMessageTypeJSONRPCErrorType:

					t.mu.RLock()
					handler := t.messageHandler
					t.mu.RUnlock()

					if handler != nil {
						handler(context.Background(), msg)
					}
				}
			}
		}
	}
}

// parseSSEMessageFromString parses a complete SSE message from a string
func (t *SSEClientTransport) parseSSEMessageFromString(message string, endpointChan chan<- struct{}) (*transport.BaseJsonRpcMessage, error) {
	lines := strings.Split(message, "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("invalid SSE message format: too few lines")
	}

	// Parse the event type
	eventType := strings.TrimPrefix(lines[0], "event: ")
	eventType = strings.TrimSpace(eventType)

	// Parse the data line
	data := strings.TrimPrefix(lines[1], "data: ")
	data = strings.TrimSpace(data)

	// Initialize message
	var msg transport.BaseJsonRpcMessage

	// Handle endpoint events differently
	if eventType == "endpoint" {
		t.postEndpoint = data
		if endpointChan != nil {
			endpointChan <- struct{}{}
		}
		return &msg, nil
	}

	// For message events, try to unmarshal as JSON RPC
	var response transport.BaseJSONRPCResponse
	if err := json.Unmarshal([]byte(data), &response); err == nil {
		msg.Type = transport.BaseMessageTypeJSONRPCResponseType
		msg.JsonRpcResponse = &response
		return &msg, nil
	}

	// Try as an error
	var errorResponse transport.BaseJSONRPCError
	if err := json.Unmarshal([]byte(data), &errorResponse); err == nil {
		msg.Type = transport.BaseMessageTypeJSONRPCErrorType
		msg.JsonRpcError = &errorResponse
		return &msg, nil
	}

	// Try as a notification
	var notification transport.BaseJSONRPCNotification
	if err := json.Unmarshal([]byte(data), &notification); err == nil {
		msg.Type = transport.BaseMessageTypeJSONRPCNotificationType
		msg.JsonRpcNotification = &notification
		return &msg, nil
	}

	return nil, fmt.Errorf("unrecognized message type: %s", eventType)
}

// Send implements Transport.Send
func (t *SSEClientTransport) Send(ctx context.Context, message *transport.BaseJsonRpcMessage) error {
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if t.postEndpoint == "" {
		return fmt.Errorf("post endpoint not set. sse connection not established")
	}

	url, err := url.JoinPath(t.baseURL, t.postEndpoint)
	if err != nil {
		return fmt.Errorf("failed to construct URL: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}

	// Handle notifications differently with a shorter timeout
	if message.Type == transport.BaseMessageTypeJSONRPCNotificationType {
		resp, err := t.notificationClient.Do(req)
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.mu.RLock()
			if t.errorHandler != nil {
				t.errorHandler(fmt.Errorf("notification error: %w", err))
			}
			t.mu.RUnlock()
		}
		if resp != nil {
			defer resp.Body.Close()
		}
		return nil
	}

	// For non-notifications, continue with normal synchronous request
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned error: %s (status: %d)", string(body), resp.StatusCode)
	}

	// Response content we care about will come in over the SSE stream.
	// We are done, here.

	return nil
}

// Close implements Transport.Close
func (t *SSEClientTransport) Close() error {
	close(t.done)
	if t.closeHandler != nil {
		t.closeHandler()
	}
	return nil
}

// SetCloseHandler implements Transport.SetCloseHandler
func (t *SSEClientTransport) SetCloseHandler(handler func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closeHandler = handler
}

// SetErrorHandler implements Transport.SetErrorHandler
func (t *SSEClientTransport) SetErrorHandler(handler func(error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.errorHandler = handler
}

// SetMessageHandler implements Transport.SetMessageHandler
func (t *SSEClientTransport) SetMessageHandler(handler func(ctx context.Context, message *transport.BaseJsonRpcMessage)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.messageHandler = handler
}
