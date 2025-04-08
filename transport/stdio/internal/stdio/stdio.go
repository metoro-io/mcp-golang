// This file implements the stdio transport layer for JSON-RPC communication.
// It provides functionality to read and write JSON-RPC messages over standard input/output
// streams, similar to the TypeScript implementation in @typescript-sdk/src/shared/stdio.ts.
//
// Key Components:
//
// 1. ReadBuffer:
//   - Buffers continuous stdio stream into discrete JSON-RPC messages
//   - Thread-safe with mutex protection
//   - Handles message framing using newline delimiters
//   - Methods: Append (add data), ReadMessage (read complete message), Clear (reset buffer)
//
// 2. StdioTransport:
//   - Implements the Transport interface using stdio
//   - Uses bufio.Reader for efficient buffered reading
//   - Thread-safe with mutex protection
//   - Supports:
//   - Asynchronous message reading
//   - Message sending with newline framing
//   - Proper cleanup on close
//   - Event handlers for close, error, and message events
//
// 3. Message Handling:
//   - Deserializes JSON-RPC messages into appropriate types:
//   - JSONRPCRequest: Messages with ID and method
//   - JSONRPCNotification: Messages with method but no ID
//   - JSONRPCError: Error responses with ID
//   - Generic responses: Success responses with ID
//   - Serializes messages to JSON with newline termination
//
// Thread Safety:
//   - All public methods are thread-safe
//   - Uses sync.Mutex for state protection
//   - Safe for concurrent reading and writing
//
// Usage:
//
//	transport := NewStdioTransport()
//	transport.SetMessageHandler(func(msg interface{}) {
//	    // Handle incoming message
//	})
//	transport.Start()
//	defer transport.Close()
//
//	// Send a message
//	transport.Send(map[string]interface{}{
//	    "jsonrpc": "2.0",
//	    "method": "test",
//	    "params": map[string]interface{}{},
//	})
//
// Error Handling:
//   - All methods return meaningful errors
//   - Transport supports error handler for async errors
//   - Proper cleanup on error conditions
//
// For more details, see the test file stdio_test.go.
package stdio

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/metoro-io/mcp-golang/transport"
)

// ReadBuffer buffers a continuous stdio stream into discrete JSON-RPC messages.
type ReadBuffer struct {
	mu     sync.Mutex
	buffer []byte
}

// NewReadBuffer creates a new ReadBuffer.
func NewReadBuffer() *ReadBuffer {
	return &ReadBuffer{}
}

// Append adds a chunk of data to the buffer.
func (rb *ReadBuffer) Append(chunk []byte) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.buffer == nil {
		rb.buffer = chunk
	} else {
		rb.buffer = append(rb.buffer, chunk...)
	}
}

// ReadMessage reads a complete JSON-RPC message from the buffer.
// Returns nil if no complete message is available.
func (rb *ReadBuffer) ReadMessage() (*transport.BaseJsonRpcMessage, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.buffer == nil {
		return nil, nil
	}

	// Find newline
	for i := 0; i < len(rb.buffer); i++ {
		if rb.buffer[i] == '\n' {
			// Extract line
			line := string(rb.buffer[:i])
			// println("read line: ", line)
			rb.buffer = rb.buffer[i+1:]
			return deserializeMessageV2(line)
		}
	}

	return nil, nil
}

// Clear clears the buffer.
func (rb *ReadBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.buffer = nil
}

// deserializeMessage deserializes a JSON-RPC message from a string.
func deserializeMessage(line string) (*transport.BaseJsonRpcMessage, error) {
	var request transport.BaseJSONRPCRequest
	if err := json.Unmarshal([]byte(line), &request); err == nil {
		// println("unmarshaled request:", spew.Sdump(request))
		return transport.NewBaseMessageRequest(&request), nil
	} else {
		// println("unmarshaled request error:", err.Error())
	}

	var notification transport.BaseJSONRPCNotification
	if err := json.Unmarshal([]byte(line), &notification); err == nil {
		return transport.NewBaseMessageNotification(&notification), nil
	} else {
		// println("unmarshaled notification error:", err.Error())
	}

	var response transport.BaseJSONRPCResponse
	if err := json.Unmarshal([]byte(line), &response); err == nil {
		return transport.NewBaseMessageResponse(&response), nil
	} else {
		// println("unmarshaled response error:", err.Error())
	}

	var errorResponse transport.BaseJSONRPCError
	if err := json.Unmarshal([]byte(line), &errorResponse); err == nil {
		return transport.NewBaseMessageError(&errorResponse), nil
	} else {
		// println("unmarshaled error response error:", err.Error())
	}

	// Must be a response
	return nil, errors.New("failed to unmarshal JSON-RPC message, unrecognized type")
}

// deserializeMessageV2 deserializes a JSON-RPC message from a string.
func deserializeMessageV2(line string) (*transport.BaseJsonRpcMessage, error) {
	var tmp transport.JSONRPCCommon
	if err := json.Unmarshal([]byte(line), &tmp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	switch {
	// 请求消息（包含 Method 和 Id）
	case tmp.Method != "" && tmp.Id != nil:
		request := transport.BaseJSONRPCRequest{
			Id:      *tmp.Id,
			Jsonrpc: tmp.Jsonrpc,
			Method:  tmp.Method,
			Params:  tmp.Params,
		}
		return transport.NewBaseMessageRequest(&request), nil

	// 通知消息（包含 Method 但不包含 Id）
	case tmp.Method != "" && tmp.Id == nil:
		notification := transport.BaseJSONRPCNotification{
			Jsonrpc: tmp.Jsonrpc,
			Method:  tmp.Method,
			Params:  tmp.Params,
		}
		return transport.NewBaseMessageNotification(&notification), nil

	// 响应消息（包含 Result 和 Id）
	case tmp.Result != nil && tmp.Id != nil:
		response := transport.BaseJSONRPCResponse{
			Id:      *tmp.Id,
			Jsonrpc: tmp.Jsonrpc,
			Result:  tmp.Result,
		}
		return transport.NewBaseMessageResponse(&response), nil

	// 错误响应消息（包含 Error 和 Id）
	case tmp.Error != nil && tmp.Id != nil:
		errorResponse := transport.BaseJSONRPCError{
			Error:   *tmp.Error,
			Id:      *tmp.Id,
			Jsonrpc: tmp.Jsonrpc,
		}
		return transport.NewBaseMessageError(&errorResponse), nil

	default:
		return nil, errors.New("failed to unmarshal JSON-RPC message, unrecognized type")
	}
}
