package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/metoro-io/mcp-golang/transport"
)

// baseTransport implements the common functionality for HTTP-based transports
type baseTransport struct {
	messageHandler func(ctx context.Context, message *transport.BaseJsonRpcMessage)
	errorHandler   func(error)
	closeHandler   func()
	mu             sync.RWMutex
	responseMap    map[int64]chan *transport.BaseJsonRpcMessage
}

func newBaseTransport() *baseTransport {
	return &baseTransport{
		responseMap: make(map[int64]chan *transport.BaseJsonRpcMessage),
	}
}

// Send implements Transport.Send
func (t *baseTransport) Send(ctx context.Context, message *transport.BaseJsonRpcMessage) error {
	key := message.JsonRpcResponse.Id
	responseChannel := t.responseMap[int64(key)]
	if responseChannel == nil {
		return fmt.Errorf("no response channel found for key: %d", key)
	}
	responseChannel <- message
	return nil
}

// Close implements Transport.Close
func (t *baseTransport) Close() error {
	if t.closeHandler != nil {
		t.closeHandler()
	}
	return nil
}

// SetCloseHandler implements Transport.SetCloseHandler
func (t *baseTransport) SetCloseHandler(handler func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closeHandler = handler
}

// SetErrorHandler implements Transport.SetErrorHandler
func (t *baseTransport) SetErrorHandler(handler func(error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.errorHandler = handler
}

// SetMessageHandler implements Transport.SetMessageHandler
func (t *baseTransport) SetMessageHandler(handler func(ctx context.Context, message *transport.BaseJsonRpcMessage)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.messageHandler = handler
}

// handleMessage processes an incoming message and returns a response
func (t *baseTransport) handleMessage(ctx context.Context, body []byte) (*transport.BaseJsonRpcMessage, error) {
	// Lock mutex to store the response writer for later use
	t.mu.Lock()
	var key int64 = 0

	// Find an unused key in the response map
	for key < 1000000 {
		if _, ok := t.responseMap[key]; !ok {
			break
		}
		key++
	}
	t.responseMap[key] = make(chan *transport.BaseJsonRpcMessage)
	t.mu.Unlock()

	var prevId *transport.RequestId = nil
	var tmp transport.JSONRPCCommon
	if err := json.Unmarshal(body, &tmp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Process the message based on its type
	switch {
	// Request message (contains Method and Id)
	case tmp.Method != "" && tmp.Id != nil:
		// Modify the request Id to the key
		originalId := *tmp.Id
		*tmp.Id = transport.RequestId(key)

		request := transport.BaseJSONRPCRequest{
			Id:      *tmp.Id,
			Jsonrpc: tmp.Jsonrpc,
			Method:  tmp.Method,
			Params:  tmp.Params,
		}

		// Call the message handler
		t.mu.RLock()
		handler := t.messageHandler
		t.mu.RUnlock()

		if handler != nil {
			handler(ctx, transport.NewBaseMessageRequest(&request))
		}

		// Set prevId to restore the original Id in the response
		prevId = &originalId

	// Notification message (contains Method but no Id)
	case tmp.Method != "" && tmp.Id == nil:
		notification := transport.BaseJSONRPCNotification{
			Jsonrpc: tmp.Jsonrpc,
			Method:  tmp.Method,
			Params:  tmp.Params,
		}

		// Call the message handler
		t.mu.RLock()
		handler := t.messageHandler
		t.mu.RUnlock()

		if handler != nil {
			handler(ctx, transport.NewBaseMessageNotification(&notification))
		}

	// Response message (contains Result and Id)
	case tmp.Result != nil && tmp.Id != nil:
		response := transport.BaseJSONRPCResponse{
			Id:      *tmp.Id,
			Jsonrpc: tmp.Jsonrpc,
			Result:  tmp.Result,
		}

		// Send response to the corresponding response channel
		t.mu.RLock()
		responseChan, exists := t.responseMap[int64(*tmp.Id)]
		t.mu.RUnlock()
		if exists {
			responseMsg := transport.NewBaseMessageResponse(&response)
			responseChan <- responseMsg
		} else {
			return nil, fmt.Errorf("no matching request for response with id %d", *tmp.Id)
		}

	// Error response message (contains Error and Id)
	case tmp.Error != nil && tmp.Id != nil:
		errorResponse := transport.BaseJSONRPCError{
			Error:   *tmp.Error,
			Id:      *tmp.Id,
			Jsonrpc: tmp.Jsonrpc,
		}

		// Send error response to the corresponding response channel
		t.mu.RLock()
		responseChan, exists := t.responseMap[int64(*tmp.Id)]
		t.mu.RUnlock()
		if exists {
			responseMsg := transport.NewBaseMessageError(&errorResponse)
			responseChan <- responseMsg
		} else {
			return nil, fmt.Errorf("no matching request for error response with id %d", *tmp.Id)
		}

	default:
		return nil, fmt.Errorf("failed to unmarshal JSON-RPC message, unrecognized type")
	}

	// If it's a request message, wait for the response from the handler
	if prevId != nil {
		responseToUse := <-t.responseMap[key]
		t.mu.Lock()
		delete(t.responseMap, key)
		t.mu.Unlock()

		// Restore the original Id in the response
		if responseToUse != nil && responseToUse.JsonRpcResponse != nil {
			responseToUse.JsonRpcResponse.Id = *prevId
		}

		return responseToUse, nil
	}

	// For notifications, responses, and error responses, no need to wait for a return
	return nil, nil
}

// readBody reads and returns the body from an io.Reader
func (t *baseTransport) readBody(reader io.Reader) ([]byte, error) {
	body, err := io.ReadAll(reader)
	if err != nil {
		if t.errorHandler != nil {
			t.errorHandler(fmt.Errorf("failed to read request body: %w", err))
		}
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	return body, nil
}
