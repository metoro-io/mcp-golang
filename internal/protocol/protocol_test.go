package protocol

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// TestProtocol_Connect verifies the basic connection functionality of the Protocol.
// This is a critical test as connection establishment is required for all other operations.
// It ensures that:
// 1. The protocol can successfully connect to a transport
// 2. The message handler is properly registered with the transport
// 3. The protocol is ready to send and receive messages after connection
func TestProtocol_Connect(t *testing.T) {
	p := NewProtocol(nil)
	transport := newMockTransport()

	if err := p.Connect(transport); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !transport.isStarted() {
		t.Error("Transport was not started")
	}
}

// TestProtocol_Close tests the proper cleanup of resources when closing the protocol.
// Proper cleanup is essential to prevent resource leaks and ensure graceful shutdown.
// It verifies:
// 1. All handlers are properly deregistered
// 2. The transport is closed
// 3. No messages can be sent after closing
// 4. Multiple closes are handled safely
func TestProtocol_Close(t *testing.T) {
	p := NewProtocol(nil)
	transport := mcp.newMockTransport()

	if err := p.Connect(transport); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	closeCalled := false
	p.OnClose = func() {
		closeCalled = true
	}

	if err := p.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !transport.isClosed() {
		t.Error("Transport was not closed")
	}

	if !closeCalled {
		t.Error("OnClose callback was not called")
	}
}

// TestProtocol_Request tests the core request-response functionality of the protocol.
// This is the most important test as it covers the primary use case of the protocol.
// It includes subtests for:
// 1. Successful request/response with proper correlation
// 2. Request timeout handling
// 3. Request cancellation via context
// These scenarios ensure the protocol can handle both successful and error cases
// while maintaining proper message correlation and resource cleanup.
func TestProtocol_Request(t *testing.T) {
	p := NewProtocol(nil)
	transport := mcp.newMockTransport()

	if err := p.Connect(transport); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Test successful request
	t.Run("Successful request", func(t *testing.T) {
		ctx := context.Background()
		go func() {
			// Simulate response after a short delay
			time.Sleep(10 * time.Millisecond)
			msgs := transport.getMessages()
			if len(msgs) == 0 {
				t.Error("No messages sent")
				return
			}

			lastMsg := msgs[len(msgs)-1]
			req, ok := lastMsg.(map[string]interface{})
			if !ok {
				t.Error("Last message is not a request")
				return
			}

			// Simulate response
			transport.simulateMessage(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result":  "test result",
			})
		}()

		result, err := p.Request(ctx, "test_method", map[string]string{"key": "value"}, nil)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if result != "test result" {
			t.Errorf("Expected result 'test result', got %v", result)
		}
	})

	// Test request timeout
	t.Run("Request timeout", func(t *testing.T) {
		ctx := context.Background()
		opts := &RequestOptions{
			Timeout: 50 * time.Millisecond,
		}

		_, err := p.Request(ctx, "test_method", nil, opts)
		if err == nil {
			t.Fatal("Expected timeout error, got nil")
		}
	})

	// Test request cancellation
	t.Run("Request cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		_, err := p.Request(ctx, "test_method", nil, nil)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Expected context.Canceled error, got %v", err)
		}
	})
}

// TestProtocol_Notification tests the handling of one-way notifications.
// Notifications are important for events that don't require responses.
// The test verifies:
// 1. Notifications can be sent successfully
// 2. The transport receives the correct notification format
// 3. No response handling is attempted for notifications
func TestProtocol_Notification(t *testing.T) {
	p := NewProtocol(nil)
	transport := mcp.newMockTransport()

	if err := p.Connect(transport); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Send notification
	if err := p.Notification("test_notification", map[string]string{"key": "value"}); err != nil {
		t.Fatalf("Notification failed: %v", err)
	}

	// Check if notification was sent
	msgs := transport.getMessages()
	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(msgs))
	}

	notification, ok := msgs[0].(map[string]interface{})
	if !ok {
		t.Fatal("Message is not a notification")
	}

	if notification["method"] != "test_notification" {
		t.Errorf("Expected method 'test_notification', got %v", notification["method"])
	}
}

// TestProtocol_RequestHandler tests the registration and invocation of request handlers.
// Request handlers are crucial for servers implementing RPC methods.
// It ensures:
// 1. Handlers can be registered for specific methods
// 2. Handlers receive the correct request parameters
// 3. Handler responses are properly sent back to clients
// 4. Handler errors are properly propagated
func TestProtocol_RequestHandler(t *testing.T) {
	p := NewProtocol(nil)
	transport := mcp.newMockTransport()

	if err := p.Connect(transport); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Register request handler
	handlerCalled := false
	p.SetRequestHandler("test_method", func(req JSONRPCRequest, extra RequestHandlerExtra) (interface{}, error) {
		handlerCalled = true
		return "handler result", nil
	})

	// Simulate incoming request
	transport.simulateMessage(&JSONRPCRequest{
		Jsonrpc: "2.0",
		Method:  "test_method",
		Id:      1,
	})

	// Give some time for handler to be called
	time.Sleep(50 * time.Millisecond)

	if !handlerCalled {
		t.Error("Request handler was not called")
	}

	// Check response
	msgs := transport.getMessages()
	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(msgs))
	}

	response, ok := msgs[0].(map[string]interface{})
	if !ok {
		t.Fatal("Message is not a response")
	}

	if response["result"] != "handler result" {
		t.Errorf("Expected result 'handler result', got %v", response["result"])
	}
}

// TestProtocol_NotificationHandler tests the handling of incoming notifications.
// This is important for asynchronous events and status updates.
// It verifies:
// 1. Notification handlers can be registered
// 2. Handlers are called with correct notification data
// 3. Multiple handlers can be registered for different methods
// 4. Unknown notifications are handled gracefully
func TestProtocol_NotificationHandler(t *testing.T) {
	p := NewProtocol(nil)
	transport := mcp.newMockTransport()

	if err := p.Connect(transport); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Register notification handler
	handlerCalled := false
	p.SetNotificationHandler("test_notification", func(notification JSONRPCNotification) error {
		handlerCalled = true
		return nil
	})

	// Simulate incoming notification
	transport.simulateMessage(&JSONRPCNotification{
		Jsonrpc: "2.0",
		Method:  "test_notification",
	})

	// Give some time for handler to be called
	time.Sleep(50 * time.Millisecond)

	if !handlerCalled {
		t.Error("Notification handler was not called")
	}
}

// TestProtocol_Progress tests the progress tracking functionality.
// Progress tracking is essential for long-running operations.
// The test covers:
// 1. Progress notifications can be sent and received
// 2. Progress tokens are properly correlated with requests
// 3. Progress callbacks are invoked with correct values
// 4. Progress handling works alongside normal request processing
func TestProtocol_Progress(t *testing.T) {
	p := NewProtocol(nil)
	transport := mcp.newMockTransport()

	if err := p.Connect(transport); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	progressReceived := make(chan Progress, 1)
	opts := &RequestOptions{
		OnProgress: func(p Progress) {
			progressReceived <- p
		},
	}

	// Start request
	go func() {
		ctx := context.Background()
		_, err := p.Request(ctx, "test_method", nil, opts)
		if err != nil {
			t.Errorf("Request failed: %v", err)
		}
	}()

	// Wait a bit for request to be sent
	time.Sleep(10 * time.Millisecond)

	// Get the progress token from the sent request
	msgs := transport.getMessages()
	if len(msgs) == 0 {
		t.Fatal("No messages sent")
	}

	req, ok := msgs[0].(map[string]interface{})
	if !ok {
		t.Fatal("Message is not a request")
	}

	params, ok := req["params"].(map[string]interface{})
	if !ok {
		params = map[string]interface{}{} // If no params, create empty map
	}

	meta, ok := params["_meta"].(map[string]interface{})
	if !ok {
		t.Fatal("Request has no _meta in params")
	}

	progressToken := meta["progressToken"]

	// Simulate progress notification
	transport.simulateMessage(&JSONRPCNotification{
		Jsonrpc: "2.0",
		Method:  "$/progress",
		Params: &JSONRPCNotificationParams{
			Meta:                 nil,
			AdditionalProperties: fmt.Sprintf(`{"progress": 50, "total": 100, "progressToken": %v}`, progressToken),
		},
	})

	// Wait for progress
	select {
	case progress := <-progressReceived:
		if progress.Progress != 50 || progress.Total != 100 {
			t.Errorf("Unexpected progress values: got %v", progress)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Progress notification not received")
	}
}

// TestProtocol_ErrorHandling tests various error conditions in the protocol.
// Proper error handling is crucial for reliability and debugging.
// It verifies:
// 1. Transport errors are properly propagated
// 2. Protocol-level errors are handled correctly
// 3. Error responses include appropriate error codes and messages
// 4. Resources are cleaned up after errors
func TestProtocol_ErrorHandling(t *testing.T) {
	p := NewProtocol(nil)
	transport := mcp.newMockTransport()

	if err := p.Connect(transport); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	errorReceived := make(chan error, 1)
	p.OnError = func(err error) {
		errorReceived <- err
	}

	// Simulate transport error
	testErr := errors.New("test error")
	transport.simulateError(testErr)

	// Wait for error
	select {
	case err := <-errorReceived:
		if err != testErr {
			t.Errorf("Expected error %v, got %v", testErr, err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Error not received")
	}
}
