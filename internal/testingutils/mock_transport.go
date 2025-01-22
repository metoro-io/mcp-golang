package testingutils

import (
	"context"
	"sync"

	"github.com/metoro-io/mcp-golang/transport"
)

// MockTransport implements Transport interface for testing
type MockTransport struct {
	mu sync.RWMutex

	// Callbacks
	onClose   func()
	onError   func(error)
	onMessage func(ctx context.Context, message *transport.BaseJsonRpcMessage)

	// Test helpers
	messages []*transport.BaseJsonRpcMessage
	closed   bool
	started  bool
}

func NewMockTransport() *MockTransport {
	return &MockTransport{
		messages: make([]*transport.BaseJsonRpcMessage, 0),
	}
}

func (t *MockTransport) Start(ctx context.Context) error {
	t.mu.Lock()
	t.started = true
	t.mu.Unlock()
	return nil
}

func (t *MockTransport) Send(ctx context.Context, message *transport.BaseJsonRpcMessage) error {
	t.mu.Lock()
	t.messages = append(t.messages, message)
	t.mu.Unlock()
	return nil
}

func (t *MockTransport) Close() error {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()
	if t.onClose != nil {
		t.onClose()
	}
	return nil
}

func (t *MockTransport) SetCloseHandler(handler func()) {
	t.mu.Lock()
	t.onClose = handler
	t.mu.Unlock()
}

func (t *MockTransport) SetErrorHandler(handler func(error)) {
	t.mu.Lock()
	t.onError = handler
	t.mu.Unlock()
}

func (t *MockTransport) SetMessageHandler(handler func(ctx context.Context, message *transport.BaseJsonRpcMessage)) {
	t.mu.Lock()
	t.onMessage = handler
	t.mu.Unlock()
}

// Test helper methods

func (t *MockTransport) SimulateMessage(msg *transport.BaseJsonRpcMessage) {
	t.mu.RLock()
	handler := t.onMessage
	t.mu.RUnlock()
	if handler != nil {
		handler(context.Background(), msg)
	}
}

func (t *MockTransport) SimulateError(err error) {
	t.mu.RLock()
	handler := t.onError
	t.mu.RUnlock()
	if handler != nil {
		handler(err)
	}
}

func (t *MockTransport) GetMessages() []*transport.BaseJsonRpcMessage {
	t.mu.RLock()
	defer t.mu.RUnlock()
	msgs := make([]*transport.BaseJsonRpcMessage, len(t.messages))
	copy(msgs, t.messages)
	return msgs
}

func (t *MockTransport) IsClosed() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.closed
}

func (t *MockTransport) IsStarted() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.started
}
