package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/metoro-io/mcp-golang/transport"
)

func TestHTTPTransport_HandleRequest(t *testing.T) {
	// Create a new transport
	tr := NewHTTPTransport("/rpc").WithAddr(":0") // Use random port for testing

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(tr.handleRequest))
	defer server.Close()

	// Set up message handler
	messageReceived := make(chan *transport.BaseJsonRpcMessage, 1)
	tr.SetMessageHandler(func(message *transport.BaseJsonRpcMessage) {
		messageReceived <- message
	})

	// Test valid JSON-RPC request
	testMessage := &transport.BaseJSONRPCRequest{
		Jsonrpc: "2.0",
		Method:  "test.method",
		Params:  json.RawMessage(`{"key": "value"}`),
		Id:      1,
	}

	jsonData, err := json.Marshal(testMessage)
	if err != nil {
		t.Fatalf("Failed to marshal test message: %v", err)
	}

	resp, err := http.Post(server.URL+"/rpc", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		t.Fatalf("Failed to send POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}

	// Wait for message to be received
	select {
	case received := <-messageReceived:
		if received.Type != transport.BaseMessageTypeJSONRPCRequestType {
			t.Errorf("Expected message type %s, got %s", transport.BaseMessageTypeJSONRPCRequestType, received.Type)
		}
		if received.JsonRpcRequest == nil {
			t.Error("Expected JsonRpcRequest to be non-nil")
			return
		}
		if received.JsonRpcRequest.Method != testMessage.Method {
			t.Errorf("Expected method %s, got %s", testMessage.Method, received.JsonRpcRequest.Method)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for message")
	}
}

func TestHTTPTransport_InvalidMethod(t *testing.T) {
	tr := NewHTTPTransport("/rpc").WithAddr(":0")
	server := httptest.NewServer(http.HandlerFunc(tr.handleRequest))
	defer server.Close()

	resp, err := http.Get(server.URL + "/rpc")
	if err != nil {
		t.Fatalf("Failed to send GET request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status Method Not Allowed, got %v", resp.Status)
	}
}

func TestHTTPTransport_InvalidJSON(t *testing.T) {
	tr := NewHTTPTransport("/rpc").WithAddr(":0")
	server := httptest.NewServer(http.HandlerFunc(tr.handleRequest))
	defer server.Close()

	resp, err := http.Post(server.URL+"/rpc", "application/json", bytes.NewBufferString("invalid json"))
	if err != nil {
		t.Fatalf("Failed to send POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status Bad Request, got %v", resp.Status)
	}
}

func TestHTTPTransport_StartAndClose(t *testing.T) {
	tr := NewHTTPTransport("/rpc").WithAddr(":0") // Use random port
	ctx := context.Background()

	// Start the transport in a goroutine since it blocks
	go func() {
		if err := tr.Start(ctx); err != nil && err != http.ErrServerClosed {
			t.Errorf("Unexpected error starting transport: %v", err)
		}
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Test close
	if err := tr.Close(); err != nil {
		t.Errorf("Failed to close transport: %v", err)
	}
}
