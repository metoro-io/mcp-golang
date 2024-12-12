package protocol

import (
	"context"
	"github.com/metoro-io/mcp-golang/transport"
	"sync"
)

// mockTransport implements Transport interface for testing
type mockTransport struct {
	mu sync.RWMutex

	// Callbacks
	onClose   func()
	onError   func(error)
	onMessage func(message *transport.BaseJsonRpcMessage)

	// Test helpers
	messages []*transport.BaseJsonRpcMessage
	closed   bool
	started  bool
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		messages: make([]*transport.BaseJsonRpcMessage, 0),
	}
}

func (t *mockTransport) Start(ctx context.Context) error {
	t.mu.Lock()
	t.started = true
	t.mu.Unlock()
	return nil
}

func (t *mockTransport) Send(message *transport.BaseJsonRpcMessage) error {
	t.mu.Lock()
	t.messages = append(t.messages, message)
	t.mu.Unlock()
	return nil
}

func (t *mockTransport) Close() error {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()
	if t.onClose != nil {
		t.onClose()
	}
	return nil
}

func (t *mockTransport) SetCloseHandler(handler func()) {
	t.mu.Lock()
	t.onClose = handler
	t.mu.Unlock()
}

func (t *mockTransport) SetErrorHandler(handler func(error)) {
	t.mu.Lock()
	t.onError = handler
	t.mu.Unlock()
}

func (t *mockTransport) SetMessageHandler(handler func(*transport.BaseJsonRpcMessage)) {
	t.mu.Lock()
	t.onMessage = handler
	t.mu.Unlock()
}

// Test helper methods

func (t *mockTransport) simulateMessage(msg *transport.BaseJsonRpcMessage) {
	t.mu.RLock()
	handler := t.onMessage
	t.mu.RUnlock()
	if handler != nil {
		handler(msg)
	}
}

func (t *mockTransport) simulateError(err error) {
	t.mu.RLock()
	handler := t.onError
	t.mu.RUnlock()
	if handler != nil {
		handler(err)
	}
}

func (t *mockTransport) getMessages() []*transport.BaseJsonRpcMessage {
	t.mu.RLock()
	defer t.mu.RUnlock()
	msgs := make([]*transport.BaseJsonRpcMessage, len(t.messages))
	copy(msgs, t.messages)
	return msgs
}

func (t *mockTransport) isClosed() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.closed
}

func (t *mockTransport) isStarted() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.started
}
