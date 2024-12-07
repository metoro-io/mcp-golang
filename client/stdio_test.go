package client

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestStdioClient_Echo tests the stdio client with a simple echo server
func TestStdioClient_Echo(t *testing.T) {
	// Create a temporary echo script
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "echo.sh")
	
	script := `#!/bin/bash
trap 'exit 0' SIGTERM SIGINT
while IFS= read -r line; do
    echo "$line"
done
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create echo script: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	params := StdioServerParameters{
		Command: "bash",
		Args:    []string{scriptPath},
	}

	client, readChan, writeChan, err := NewStdioClient(ctx, params)
	if err != nil {
		t.Fatalf("Failed to create stdio client: %v", err)
	}
	
	// Test message
	testMsg := JSONRPCMessage{
		Method: "test",
		ID:     1,
		Params: json.RawMessage(`{"hello":"world"}`),
	}

	// Send message
	writeChan <- testMsg

	// Read response with timeout
	select {
	case msg := <-readChan:
		if msg.Method != testMsg.Method {
			t.Errorf("Expected method %q, got %q", testMsg.Method, msg.Method)
		}
		// Compare ID values as float64 since JSON numbers are decoded as float64
		expectedID := float64(testMsg.ID.(int))
		actualID, ok := msg.ID.(float64)
		if !ok {
			t.Errorf("Expected ID to be float64, got %T", msg.ID)
		} else if expectedID != actualID {
			t.Errorf("Expected ID %v, got %v", expectedID, actualID)
		}
	case <-ctx.Done():
		t.Fatal("Timeout waiting for response")
	}

	if err := client.Close(); err != nil {
		t.Errorf("Failed to close client: %v", err)
	}
}

// TestStdioClient_InvalidJSON tests handling of invalid JSON messages
func TestStdioClient_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "invalid.sh")
	
	script := `#!/bin/bash
trap 'exit 0' SIGTERM SIGINT
echo "not a json message"
echo '{"result": {"valid": "json"}}'
sleep 0.1
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	params := StdioServerParameters{
		Command: "bash",
		Args:    []string{scriptPath},
	}

	client, readChan, _, err := NewStdioClient(ctx, params)
	if err != nil {
		t.Fatalf("Failed to create stdio client: %v", err)
	}

	// Should receive only the valid JSON message
	select {
	case msg := <-readChan:
		if msg.Result == nil {
			t.Errorf("Expected result not to be nil")
			break
		}
		resultMap, ok := msg.Result.(map[string]interface{})
		if !ok {
			t.Errorf("Expected result to be a map, got %T", msg.Result)
			break
		}
		if resultMap["valid"] != "json" {
			t.Errorf("Expected valid json message, got %v", resultMap)
		}
	case <-ctx.Done():
		t.Fatal("Timeout waiting for response")
	}

	if err := client.Close(); err != nil {
		t.Errorf("Failed to close client: %v", err)
	}
}

// TestStdioClient_Environment tests environment variable passing
func TestStdioClient_Environment(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "env.sh")
	
	script := `#!/bin/bash
trap 'exit 0' SIGTERM SIGINT
echo '{"result": {"env": "'$TEST_ENV'"}}'
sleep 0.1
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	params := StdioServerParameters{
		Command: "bash",
		Args:    []string{scriptPath},
		Env:     map[string]string{"TEST_ENV": "test_value"},
	}

	client, readChan, _, err := NewStdioClient(ctx, params)
	if err != nil {
		t.Fatalf("Failed to create stdio client: %v", err)
	}

	select {
	case msg := <-readChan:
		if msg.Result == nil {
			t.Errorf("Expected result not to be nil")
			break
		}
		resultMap, ok := msg.Result.(map[string]interface{})
		if !ok {
			t.Errorf("Expected result to be a map, got %T", msg.Result)
			break
		}
		if resultMap["env"] != "test_value" {
			t.Errorf("Expected env value %q, got %q", "test_value", resultMap["env"])
		}
	case <-ctx.Done():
		t.Fatal("Timeout waiting for response")
	}

	if err := client.Close(); err != nil {
		t.Errorf("Failed to close client: %v", err)
	}
}

// TestStdioClient_Close tests proper cleanup
func TestStdioClient_Close(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "long_running.sh")
	
	script := `#!/bin/bash
trap 'exit 0' SIGTERM SIGINT
while true; do
    sleep 0.1
done
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	params := StdioServerParameters{
		Command: "bash",
		Args:    []string{scriptPath},
	}

	client, _, _, err := NewStdioClient(ctx, params)
	if err != nil {
		t.Fatalf("Failed to create stdio client: %v", err)
	}

	// Close should not block indefinitely
	done := make(chan error)
	go func() {
		done <- client.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Close failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close timed out")
	}
}
