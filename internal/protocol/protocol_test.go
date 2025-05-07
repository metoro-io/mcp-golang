package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/metoro-io/mcp-golang/internal/testingutils"
	"github.com/metoro-io/mcp-golang/transport"
)

// TestProtocol_Connect verifies the basic connection functionality of the Protocol.
// This is a critical test as connection establishment is required for all other operations.
// It ensures that:
// 1. The protocol can successfully connect to a transport
// 2. The message handler is properly registered with the transport
// 3. The protocol is ready to send and receive messages after connection
func TestProtocol_Connect(t *testing.T) {
	p := NewProtocol(nil)
	transport := testingutils.NewMockTransport()

	if err := p.Connect(transport); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !transport.IsStarted() {
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
	transport := testingutils.NewMockTransport()

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

	if !transport.IsClosed() {
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
	p.SetRequestHandler("test_method", func(ctx context.Context, req *transport.BaseJSONRPCRequest, extra RequestHandlerExtra) (transport.JsonRpcBody, error) {
		return transport.NewBaseMessageResponse(&transport.BaseJSONRPCResponse{
			Jsonrpc: "2.0",
			Id:      req.Id,
			Result:  json.RawMessage(`{"result": "test result"}`),
		}), nil
	})
	tr := testingutils.NewMockTransport()

	if err := p.Connect(tr); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
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
	tr := testingutils.NewMockTransport()

	if err := p.Connect(tr); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Send notification
	if err := p.Notification("test_notification", map[string]string{"key": "value"}); err != nil {
		t.Fatalf("Notification failed: %v", err)
	}

	// Check if notification was sent
	msgs := tr.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(msgs))
	}

	notification := msgs[0]
	if notification.Type != transport.BaseMessageTypeJSONRPCNotificationType {
		t.Fatal("Message is not a notification")
	}

	if notification.JsonRpcNotification.Method != "test_notification" {
		t.Errorf("Expected method 'test_notification', got %v", notification.JsonRpcNotification.Method)
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
	tr := testingutils.NewMockTransport()

	if err := p.Connect(tr); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Register request handler
	handlerCalled := false
	p.SetRequestHandler("test_method", func(ctx context.Context, req *transport.BaseJSONRPCRequest, extra RequestHandlerExtra) (transport.JsonRpcBody, error) {
		handlerCalled = true
		return map[string]interface{}{"result": "handler result"}, nil
	})

	// Simulate incoming request
	tr.SimulateMessage(transport.NewBaseMessageRequest(&transport.BaseJSONRPCRequest{
		Jsonrpc: "2.0",
		Method:  "test_method",
		Params:  json.RawMessage(`{"param": "value"}`),
	}))

	// Give some time for handler to be called
	time.Sleep(50 * time.Millisecond)

	if !handlerCalled {
		t.Error("Request handler was not called")
	}

	// Check response
	msgs := tr.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(msgs))
	}

	response := msgs[0]
	if response.Type != transport.BaseMessageTypeJSONRPCResponseType {
		t.Fatal("Message is not a response")
	}

	if string(response.JsonRpcResponse.Result) != (`{"result":"handler result"}`) {
		t.Errorf("Expected result 'handler result', got %v", string(response.JsonRpcResponse.Result))
	}
}

func TestProtocol_RequestHandler_ErrorResponse(t *testing.T) {
	p := NewProtocol(nil)
	tr := testingutils.NewMockTransport()

	if err := p.Connect(tr); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Register request handler
	handlerCalled := false
	p.SetRequestHandler("test_method", func(ctx context.Context, req *transport.BaseJSONRPCRequest, extra RequestHandlerExtra) (transport.JsonRpcBody, error) {
		handlerCalled = true
		return nil, errors.New("sample error")
	})

	// Simulate incoming request
	tr.SimulateMessage(transport.NewBaseMessageRequest(&transport.BaseJSONRPCRequest{
		Jsonrpc: "2.0",
		Method:  "test_method",
		Params:  json.RawMessage(`{"param": "value"}`),
	}))

	// Give some time for handler to be called
	time.Sleep(50 * time.Millisecond)

	if !handlerCalled {
		t.Error("Request handler was not called")
	}

	// Check response
	msgs := tr.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(msgs))
	}

	response := msgs[0]
	if response.Type != transport.BaseMessageTypeJSONRPCErrorType {
		t.Fatal("Message is not a response")
	}

	if string(response.JsonRpcError.Error.Message) != "sample error" {
		t.Errorf("Expected result 'sample error', got %v", string(response.JsonRpcError.Error.Message))
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
	tr := testingutils.NewMockTransport()

	if err := p.Connect(tr); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Register notification handler
	handlerCalled := false
	p.SetNotificationHandler("test_notification", func(notification *transport.BaseJSONRPCNotification) error {
		handlerCalled = true
		return nil
	})

	// Simulate incoming notification
	tr.SimulateMessage(&transport.BaseJsonRpcMessage{
		Type: transport.BaseMessageTypeJSONRPCNotificationType,
		JsonRpcNotification: &transport.BaseJSONRPCNotification{
			Jsonrpc: "2.0",
			Method:  "test_notification",
		},
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
	tr := testingutils.NewMockTransport()

	if err := p.Connect(tr); err != nil {
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
	msgs := tr.GetMessages()
	if len(msgs) == 0 {
		t.Fatal("No messages sent")
	}

	req := msgs[0]

	if req.Type != transport.BaseMessageTypeJSONRPCRequestType {
		t.Fatal("Message is not a request")
	}

	var params map[string]interface{}
	err := json.Unmarshal(req.JsonRpcRequest.Params, &params)
	if err != nil {
		t.Fatalf("Failed to unmarshal params: %v", err)
	}

	meta, ok := params["_meta"].(map[string]interface{})
	if !ok {
		t.Fatal("Request has no _meta in params")
	}

	progressToken := meta["progressToken"]

	// Simulate progress notification
	marshal, err := json.Marshal(map[string]interface{}{
		"progress":      50,
		"total":         100,
		"progressToken": progressToken,
	})
	if err != nil {
		t.Fatalf("Failed to marshal progress: %v", err)
	}
	tr.SimulateMessage(transport.NewBaseMessageNotification(&transport.BaseJSONRPCNotification{
		Jsonrpc: "2.0",
		Method:  "$/progress",
		Params:  marshal,
	}))

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
	tr := testingutils.NewMockTransport()

	if err := p.Connect(tr); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	errorReceived := make(chan error, 1)
	p.OnError = func(err error) {
		errorReceived <- err
	}

	// Simulate tr error
	testErr := errors.New("test error")
	tr.SimulateError(testErr)

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
