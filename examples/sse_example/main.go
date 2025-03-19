package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	mcp_golang "github.com/rvoh-emccaleb/mcp-golang"
	"github.com/rvoh-emccaleb/mcp-golang/transport/sse"
)

// TimeArgs defines the arguments for the time tool
type TimeArgs struct {
	Format string `json:"format" jsonschema:"description=The time format to use"`
}

const (
	baseEndpoint    = "/mcp/sse"
	protocolVersion = "2024-11-05"
	serverPort      = 8083
)

func main() {
	// Add a root context that we can cancel
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	sseTransport := sse.NewServerTransport(baseEndpoint)

	server := mcp_golang.NewServer(
		sseTransport,
		mcp_golang.WithName("mcp-golang-sse-example"),
		mcp_golang.WithVersion("0.0.1"),
	)

	// Register a simple tool
	err := server.RegisterTool("time", "Returns the current time in the specified format", func(ctx context.Context, args TimeArgs) (*mcp_golang.ToolResponse, error) {
		format := args.Format
		if format == "" {
			format = time.RFC3339
		}
		return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(time.Now().Format(format))), nil
	})
	if err != nil {
		panic(err)
	}

	// Handler for establishing a new SSE connection
	http.HandleFunc(baseEndpoint, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Create a context that's cancelled either by client disconnect or server shutdown
		ctx, cancel := context.WithCancel(r.Context())
		go func() {
			select {
			case <-rootCtx.Done():
				cancel()
			case <-r.Context().Done():
				cancel()
			}
		}()
		defer cancel()

		connID, err := sseTransport.HandleSSEConnInitialize(w)
		if err != nil {
			log.Printf("error initializing sse connection: %v", err)
			return
		}

		log.Printf("New SSE connection established with ID: %d", connID)
		start := time.Now()

		<-ctx.Done() // Wait for either client disconnect or server shutdown

		log.Printf("SSE connection closed after %v for connection with ID: %d", time.Since(start), connID)
		sseTransport.RemoveSSEConnection(connID)
	})

	// Handler for receiving JSON-RPC messages
	http.HandleFunc(baseEndpoint+"/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		log.Printf("Received POST request to %s", r.URL.Path)

		// Extract connection ID from path
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 3 {
			log.Printf("Error: missing connection ID in path")
			http.Error(w, "missing connection ID in path", http.StatusBadRequest)
			return
		}
		connIDStr := parts[len(parts)-1]

		connID, err := strconv.ParseInt(connIDStr, 10, 64)
		if err != nil {
			log.Printf("Error: invalid connection ID format: %s", connIDStr)
			http.Error(w, fmt.Sprintf("invalid connection ID format: %s", connIDStr), http.StatusBadRequest)
			return
		}

		// Read and log the request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error reading request body: %v", err)
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		log.Printf("Request body: %s", string(body))
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		// Handle all MCP messages (e.g. initialize notification, list tools request, etc.)
		if err := sseTransport.HandleMCPMessage(w, r, connID); err != nil {
			log.Printf("error handling MCP message: %v", err)
			return
		}
	})

	// Sets the MCP protocol message handlers on our transport
	// e.g. initialize, tools/list, tools/call, etc.
	err = server.Serve()
	if err != nil {
		log.Fatalf("Server error: %v", err)
	}

	// Start the HTTP server
	log.Printf("Starting HTTP server with SSE on :%d...", serverPort)
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", serverPort),
		Handler: nil,

		// Timeouts
		ReadTimeout:       10 * time.Second, // For initial connection and POST requests
		WriteTimeout:      0,                // No timeout for SSE writes
		MaxHeaderBytes:    1 << 20,          // 1MB
		ReadHeaderTimeout: 10 * time.Second, // For initial connection and POST requests
		IdleTimeout:       0,                // No timeout for SSE connections
	}

	done := make(chan struct{})

	go func() {
		err := httpServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}

		close(done)
	}()

	time.Sleep(1 * time.Second)

	// Create an HTTP transport that connects to the server
	transport := sse.NewSSEClientTransport(baseEndpoint, 1*time.Millisecond)
	transport.WithBaseURL(fmt.Sprintf("http://localhost:%d", serverPort))

	// Create a new client with the transport
	client := mcp_golang.NewClient(transport)

	// Initialize the client
	if resp, err := client.Initialize(
		context.Background(),
		&mcp_golang.InitializeRequestParams{
			ClientInfo: mcp_golang.InitializeRequestClientInfo{
				Name:    "example-client",
				Version: "0.1.0",
			},
			ProtocolVersion: protocolVersion,
			Capabilities:    nil,
		},
	); err != nil {
		log.Fatalf("Failed to initialize client: %v", err)

	} else {
		log.Printf("Initialized client: %v", spew.Sdump(resp))
	}

	// List available tools
	tools, err := client.ListTools(context.Background(), nil)
	if err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}

	log.Println("Available Tools:")
	for _, tool := range tools.Tools {
		desc := ""
		if tool.Description != nil {
			desc = *tool.Description
		}
		log.Printf("Tool: %s. Description: %s", tool.Name, desc)
	}

	// Call the time tool with different formats
	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"Mon, 02 Jan 2006",
	}

	for _, format := range formats {
		args := map[string]interface{}{
			"format": format,
		}

		response, err := client.CallTool(context.Background(), "time", args)
		if err != nil {
			log.Printf("Failed to call time tool: %v", err)
			continue
		}

		if len(response.Content) > 0 && response.Content[0].TextContent != nil {
			log.Printf("Time in format %q: %s", format, response.Content[0].TextContent.Text)
		}
	}

	// When shutting down:
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. First cancel all SSE connections and prevent new ones
	log.Println("Cancelling all SSE connections...")
	rootCancel()

	// 2. Close the transport (which should prevent new SSE connections)
	log.Println("Closing SSE transport...")
	if err := sseTransport.Close(); err != nil {
		log.Printf("Error closing SSE transport: %v", err)
	}

	// 3. Finally shutdown the HTTP server
	log.Println("Shutting down HTTP server...")
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}

	select {
	case <-done:
		log.Println("Server shutdown completed")
	case <-time.After(10 * time.Second):
		log.Println("Server shutdown timed out")
	}
}
