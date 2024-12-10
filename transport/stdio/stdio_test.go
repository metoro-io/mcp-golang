package stdio

import (
	"github.com/metoro-io/mcp-golang/transport/stdio/internal"
	"github.com/metoro-io/mcp-golang/transport/stdio/internal/stdio"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestReadBuffer tests the buffering functionality for JSON-RPC messages.
// The ReadBuffer is crucial for handling streaming input and properly framing messages.
// It verifies:
// 1. Empty buffer handling returns nil message
// 2. Incomplete messages are properly buffered
// 3. Complete messages are correctly extracted
// 4. Multiple message fragments are handled correctly
// 5. Buffer clearing works as expected
// This is a critical test as message framing is fundamental to the protocol.
func TestReadBuffer(t *testing.T) {
	rb := stdio.NewReadBuffer()

	// Test empty buffer
	msg, err := rb.ReadMessage()
	if err != nil {
		t.Errorf("ReadMessage failed: %v", err)
	}
	if msg != nil {
		t.Errorf("Expected nil message, got %v", msg)
	}

	// Test incomplete message
	rb.Append([]byte(`{"jsonrpc": "2.0", "method": "test"`))
	msg, err = rb.ReadMessage()
	if err != nil {
		t.Errorf("ReadMessage failed: %v", err)
	}
	if msg != nil {
		t.Errorf("Expected nil message, got %v", msg)
	}

	// Test complete message
	rb.Append([]byte(`, "params": {}}`))
	rb.Append([]byte("\n"))
	msg, err = rb.ReadMessage()
	if err != nil {
		t.Errorf("ReadMessage failed: %v", err)
	}
	if msg == nil {
		t.Error("Expected message, got nil")
	}

	// Test clear
	rb.Clear()
	msg, err = rb.ReadMessage()
	if err != nil {
		t.Errorf("ReadMessage failed: %v", err)
	}
	if msg != nil {
		t.Errorf("Expected nil message, got %v", msg)
	}
}

// TestMessageDeserialization tests the parsing of different JSON-RPC message types.
// Proper message type detection and parsing is critical for protocol operation.
// It tests:
// 1. Request messages (with method and ID)
// 2. Notification messages (with method, no ID)
// 3. Error responses (with error object)
// 4. Success responses (with result)
// Each message type must be correctly identified and parsed to maintain protocol integrity.
func TestMessageDeserialization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType interface{}
	}{
		{
			name:     "request",
			input:    `{"jsonrpc": "2.0", "method": "test", "params": {}, "id": 1}`,
			wantType: &JSONRPCRequest{},
		},
		{
			name:     "notification",
			input:    `{"jsonrpc": "2.0", "method": "test", "params": {}}`,
			wantType: &JSONRPCNotification{},
		},
		{
			name:     "error",
			input:    `{"jsonrpc": "2.0", "error": {"code": -32600, "message": "Invalid Request"}, "id": 1}`,
			wantType: &JSONRPCError{},
		},
		{
			name:     "response",
			input:    `{"jsonrpc": "2.0", "result": {}, "id": 1}`,
			wantType: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := internal.deserializeMessage(tt.input)
			if err != nil {
				t.Errorf("deserializeMessage failed: %v", err)
			}
			if msg == nil {
				t.Error("Expected message, got nil")
			}
			switch tt.wantType.(type) {
			case *JSONRPCRequest:
				if _, ok := msg.(*JSONRPCRequest); !ok {
					t.Errorf("Expected *mcp.JSONRPCRequest, got %T", msg)
				}
			case *JSONRPCNotification:
				if _, ok := msg.(*JSONRPCNotification); !ok {
					t.Errorf("Expected *mcp.JSONRPCNotification, got %T", msg)
				}
			case *JSONRPCError:
				if _, ok := msg.(*JSONRPCError); !ok {
					t.Errorf("Expected *mcp.JSONRPCError, got %T", msg)
				}
			case map[string]interface{}:
				if _, ok := msg.(map[string]interface{}); !ok {
					t.Errorf("Expected map[string]interface{}, got %T", msg)
				}
			}
		})
	}

	t.Run("request", func(t *testing.T) {
		msg, err := internal.deserializeMessage(`{"jsonrpc":"2.0","id":1,"method":"test","params":{}}`)
		assert.NoError(t, err)
		req, ok := msg.(*JSONRPCRequest)
		assert.True(t, ok)
		assert.Equal(t, "2.0", req.Jsonrpc)
	})

	t.Run("notification", func(t *testing.T) {
		msg, err := internal.deserializeMessage(`{"jsonrpc":"2.0","method":"test","params":{}}`)
		assert.NoError(t, err)
		notif, ok := msg.(*JSONRPCNotification)
		assert.True(t, ok)
		assert.Equal(t, "2.0", notif.Jsonrpc)
	})

	t.Run("error", func(t *testing.T) {
		msg, err := internal.deserializeMessage(`{"jsonrpc":"2.0","id":1,"error":{"code":-32700,"message":"Parse error"}}`)
		assert.NoError(t, err)
		errMsg, ok := msg.(*JSONRPCError)
		assert.True(t, ok)
		assert.Equal(t, "2.0", errMsg.Jsonrpc)
	})
}
