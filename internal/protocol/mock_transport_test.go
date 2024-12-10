package protocol

import (
	"context"
	"sync"
)

// mockTransport implements Transport interface for testing
type mockTransport struct {
	mu sync.RWMutex

	// Callbacks
	onClose   func()
	onError   func(error)
	onMessage func(JSONRPCMessage)

	// Test helpers
	messages []JSONRPCMessage
	closed   bool
	started  bool
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		messages: make([]JSONRPCMessage, 0),
	}
}

func (t *mockTransport) Start(ctx context.Context) error {
	t.mu.Lock()
	t.started = true
	t.mu.Unlock()
	return nil
}

func (t *mockTransport) Send(message JSONRPCMessage) error {
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

func (t *mockTransport) SetMessageHandler(handler func(JSONRPCMessage)) {
	t.mu.Lock()
	t.onMessage = handler
	t.mu.Unlock()
}

// Test helper methods

func (t *mockTransport) simulateMessage(msg JSONRPCMessage) {
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

func (t *mockTransport) getMessages() []JSONRPCMessage {
	t.mu.RLock()
	defer t.mu.RUnlock()
	msgs := make([]JSONRPCMessage, len(t.messages))
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
