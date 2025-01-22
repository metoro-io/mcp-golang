# Gin Integration Example

This example demonstrates how to integrate the MCP server with a Gin web application. It shows how to:
1. Create an MCP server with a Gin transport
2. Register tools with the server
3. Add the MCP endpoint to a Gin router

## Running the Example

1. Start the server:
   ```bash
   go run main.go
   ```
   This will start a Gin server on port 8081 with an MCP endpoint at `/mcp`.

2. You can test it using the HTTP client example:
   ```bash
   cd ../http_example
   go run client/main.go
   ```

## Understanding the Code

The key components are:

1. `GinTransport`: A transport implementation that works with Gin's router
2. `Handler()`: Returns a Gin handler function that can be used with any Gin router
3. Tool Registration: Shows how to register tools that can be called via the MCP endpoint

## Integration with Existing Gin Applications

To add MCP support to your existing Gin application:

```go
// Create the transport
transport := http.NewGinTransport()

// Create the MCP server
server := mcp_golang.NewServer(transport)

// Register your tools
server.RegisterTool("mytool", "Tool description", myToolHandler)

// Start the server
go server.Serve()

// Add the MCP endpoint to your Gin router
router.POST("/mcp", transport.Handler())
``` 