package base

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/rvoh-emccaleb/mcp-golang/transport"
)

// Transport implements the common functionality for transports
type Transport struct {
	messageHandler func(ctx context.Context, message *transport.BaseJsonRpcMessage)
	errorHandler   func(error)
	closeHandler   func()
	mu             sync.RWMutex
	ResponseMap    map[int64]chan *transport.BaseJsonRpcMessage
}

// NewTransport creates a new base transport
func NewTransport() *Transport {
	return &Transport{
		ResponseMap: make(map[int64]chan *transport.BaseJsonRpcMessage),
	}
}

// Send implements Transport.Send
func (t *Transport) Send(ctx context.Context, message *transport.BaseJsonRpcMessage) error {
	key := message.JsonRpcResponse.Id
	responseChannel := t.ResponseMap[int64(key)]
	if responseChannel == nil {
		return fmt.Errorf("no response channel found for key: %d", key)
	}
	responseChannel <- message
	return nil
}

// Close implements Transport.Close
func (t *Transport) Close() error {
	if t.closeHandler != nil {
		t.closeHandler()
	}
	return nil
}

// SetCloseHandler implements Transport.SetCloseHandler
func (t *Transport) SetCloseHandler(handler func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closeHandler = handler
}

// SetErrorHandler implements Transport.SetErrorHandler
func (t *Transport) SetErrorHandler(handler func(error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.errorHandler = handler
}

// SetMessageHandler implements Transport.SetMessageHandler
func (t *Transport) SetMessageHandler(handler func(ctx context.Context, message *transport.BaseJsonRpcMessage)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.messageHandler = handler
}

// HandleMessage processes an incoming message and returns a response
func (t *Transport) HandleMessage(ctx context.Context, body []byte) (*transport.BaseJsonRpcMessage, error) {
	// Try to unmarshal as a request first
	var request transport.BaseJSONRPCRequest
	if err := json.Unmarshal(body, &request); err == nil {
		// Create a response channel for this request
		t.mu.Lock()
		var key int64 = 0
		for key < 1000000 {
			if _, ok := t.ResponseMap[key]; !ok {
				break
			}
			key = key + 1
		}
		t.ResponseMap[key] = make(chan *transport.BaseJsonRpcMessage)
		t.mu.Unlock()

		originalID := request.Id
		request.Id = transport.RequestId(key)
		t.mu.RLock()
		handler := t.messageHandler
		t.mu.RUnlock()

		if handler != nil {
			handler(ctx, transport.NewBaseMessageRequest(&request))
		}

		// Block until the response is received
		responseToUse := <-t.ResponseMap[key]
		delete(t.ResponseMap, key)

		// Restore the original client ID in the response
		responseToUse.JsonRpcResponse.Id = originalID
		return responseToUse, nil
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
		return nil, nil
	}

	// Try as a response
	var response transport.BaseJSONRPCResponse
	if err := json.Unmarshal(body, &response); err == nil {
		t.mu.RLock()
		handler := t.messageHandler
		t.mu.RUnlock()

		if handler != nil {
			handler(ctx, transport.NewBaseMessageResponse(&response))
		}
		return nil, nil
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
		return nil, nil
	}

	return nil, fmt.Errorf("failed to unmarshal JSON-RPC message, unrecognized type")
}
