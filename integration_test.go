// Package mcp_golang provides integration tests for the MCP (Machine Control Protocol) server implementation.
// This file contains end-to-end tests that verify the server's functionality by running a real server process
// and communicating with it through stdio transport.

package mcp_golang

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/metoro-io/mcp-golang/internal/protocol"
	"github.com/metoro-io/mcp-golang/transport"
	mcphttp "github.com/metoro-io/mcp-golang/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testServerCode = `package main

import (
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

type EchoArgs struct {
	Message string ` + "`json:\"message\" jsonschema:\"required,description=Message to echo back\"`" + `
}

func main() {
	server := mcp.NewServer(stdio.NewStdioServerTransport())
	err := server.RegisterTool("echo", "Echo back the input message", func(args EchoArgs) (*mcp.ToolResponse, error) {
		return mcp.NewToolResponse(mcp.NewTextContent(args.Message)), nil
	})
	if err != nil {
		panic(err)
	}

	err = server.Serve()
	if err != nil {
		panic(err)
	}

	select {}
}
`

// testServerCode contains a simple echo server implementation used for testing.
// It registers a single "echo" tool that returns the input message.

var i = 1

// TestServerIntegration performs an end-to-end test of the MCP server functionality.
// The test follows these steps:
// 1. Sets up a temporary Go module and builds a test server
// 2. Starts the server process with stdio communication
// 3. Tests server initialization
// 4. Tests tool listing functionality
// 5. Tests the echo tool by sending and receiving messages
func TestServerIntegration(t *testing.T) {
	// Get the current module's root directory
	currentDir, err := os.Getwd()
	require.NoError(t, err)

	// Create a temporary directory for our test server
	tmpDir, err := os.MkdirTemp("", "mcp-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Initialize a new module
	cmd := exec.Command("go", "mod", "init", "testserver")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to initialize module: %s", string(output))

	// Replace the dependency with the local version
	cmd = exec.Command("go", "mod", "edit", "-replace", "github.com/metoro-io/mcp-golang="+currentDir)
	cmd.Dir = tmpDir
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "Failed to replace dependency: %s", string(output))

	// Write the test server code
	serverPath := filepath.Join(tmpDir, "test_server.go")
	err = os.WriteFile(serverPath, []byte(testServerCode), 0644)
	require.NoError(t, err)

	// Run go mod tidy
	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = tmpDir
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "Failed to tidy modules: %s", string(output))

	// Build the test server
	binPath := filepath.Join(tmpDir, "test_server")
	cmd = exec.Command("go", "build", "-o", binPath, serverPath)
	cmd.Dir = tmpDir
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build test server: %s\nServer code:\n%s", string(output), testServerCode)

	// Start the server process
	serverProc := exec.Command(binPath)
	stdin, err := serverProc.StdinPipe()
	require.NoError(t, err)
	stdout, err := serverProc.StdoutPipe()
	require.NoError(t, err)
	stderr, err := serverProc.StderrPipe()
	require.NoError(t, err)

	err = serverProc.Start()
	require.NoError(t, err)
	defer serverProc.Process.Kill()

	// Start a goroutine to read stderr
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				if err != io.EOF {
					t.Logf("Error reading stderr: %v", err)
				}
				return
			}
			if n > 0 {
				t.Logf("Server stderr: %s", string(buf[:n]))
			}
		}
	}()

	// Helper function to send a request and read response
	sendRequest := func(method string, params interface{}) (map[string]interface{}, error) {
		paramsBytes, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}

		req := transport.BaseJSONRPCRequest{
			Jsonrpc: "2.0",
			Method:  method,
			Params:  json.RawMessage(paramsBytes),
			Id:      transport.RequestId(i),
		}
		i++

		reqBytes, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}
		reqBytes = append(reqBytes, '\n')

		t.Logf("Sending request: %s", string(reqBytes))
		_, err = stdin.Write(reqBytes)
		if err != nil {
			return nil, err
		}

		// Read response with timeout
		respChan := make(chan map[string]interface{}, 1)
		errChan := make(chan error, 1)

		go func() {
			var buf bytes.Buffer
			reader := io.TeeReader(stdout, &buf)
			decoder := json.NewDecoder(reader)

			t.Log("Waiting for response...")
			var response map[string]interface{}
			err := decoder.Decode(&response)
			if err != nil {
				errChan <- fmt.Errorf("failed to decode response: %v\nraw response: %s", err, buf.String())
				return
			}
			t.Logf("Got response: %+v", response)
			respChan <- response
		}()

		select {
		case resp := <-respChan:
			return resp, nil
		case err := <-errChan:
			return nil, err
		case <-time.After(5 * time.Second): // Increased timeout to 5 seconds
			return nil, fmt.Errorf("timeout waiting for response")
		}
	}

	// Test 1: Initialize
	resp, err := sendRequest("initialize", map[string]interface{}{
		"capabilities": map[string]interface{}{},
	})
	require.NoError(t, err)
	assert.Equal(t, float64(1), resp["id"])
	assert.NotNil(t, resp["result"])

	time.Sleep(100 * time.Millisecond)

	// Test 2: List tools
	resp, err = sendRequest("tools/list", map[string]interface{}{})
	require.NoError(t, err)
	tools, ok := resp["result"].(map[string]interface{})["tools"].([]interface{})
	require.True(t, ok)
	assert.Len(t, tools, 1)
	tool := tools[0].(map[string]interface{})
	assert.Equal(t, "echo", tool["name"])

	// Test 3: Call echo tool
	callParams := map[string]interface{}{
		"name": "echo",
		"arguments": map[string]interface{}{
			"message": "Hello, World!",
		},
	}
	resp, err = sendRequest("tools/call", callParams)
	require.NoError(t, err)
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "Hello, World!", content["text"])
}

func TestReadResource(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	uri := "test://example"
	resourceText := `{"app_name":"MyExampleServer","version":"1.0.0"}`
	mimeType := "application/json"

	// Start server on a random port
	server, addr := startTestServerWithResource(t, uri, resourceText, mimeType)
	defer server.Close()

	// Create HTTP client transport
	clientTransport := mcphttp.NewHTTPClientTransport("/mcp")
	clientTransport.WithBaseURL(fmt.Sprintf("http://%s", addr))

	// Create client
	client := NewClient(clientTransport)

	// Initialize
	_, err := client.Initialize(context.Background())
	assert.NoError(t, err)

	// Call ReadResource
	response, err := client.ReadResource(context.Background(), uri)

	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 1, len(response.Contents))
	assert.NotNil(t, response.Contents[0].TextResourceContents)
	assert.Equal(t, uri, response.Contents[0].TextResourceContents.Uri)
	assert.Equal(t, resourceText, response.Contents[0].TextResourceContents.Text)
}

// Helper function to start a test server with a resource
func startTestServerWithResource(t *testing.T, uri, text, mimeType string) (*http.Server, string) {
	// Create HTTP transport
	serverTransport := mcphttp.NewHTTPTransport("/mcp")

	// Create temporary server
	tempAddr := "localhost:0" // Let the OS choose a free port
	serverTransport.WithAddr(tempAddr)

	// Create server and register the resource
	server := NewServer(serverTransport)

	err := server.RegisterResource(
		uri,
		"example",
		"Example resource for testing",
		mimeType,
		func() (*ResourceResponse, error) {
			embeddedResource := NewTextEmbeddedResource(uri, text, mimeType)
			return NewResourceResponse(embeddedResource), nil
		},
	)
	assert.NoError(t, err)

	// Start an HTTP server to handle the actual traffic
	mux := http.NewServeMux()
	httpServer := &http.Server{Handler: mux}

	listener, err := net.Listen("tcp", tempAddr)
	assert.NoError(t, err)

	// Create a handler that forwards requests to our server
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Parse the request
		var request transport.BaseJSONRPCRequest
		err = json.Unmarshal(body, &request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Process the request with our server
		ctx := context.Background()
		switch request.Method {
		case "initialize":
			result, err := server.handleInitialize(ctx, &request, protocol.RequestHandlerExtra{})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSONResponse(w, request.Id, result)
		case "resources/read":
			result, err := server.handleResourceCalls(ctx, &request, protocol.RequestHandlerExtra{})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSONResponse(w, request.Id, result)
		default:
			http.Error(w, "Method not supported", http.StatusBadRequest)
		}
	})

	// Start server in a goroutine
	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			t.Logf("HTTP server error: %v", err)
		}
	}()

	// Return the server and the actual address it's listening on
	return httpServer, listener.Addr().String()
}

func writeJSONResponse(w http.ResponseWriter, id interface{}, result interface{}) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}

	json.NewEncoder(w).Encode(response)
}
