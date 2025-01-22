# HTTP Transport Example

This example demonstrates how to use the HTTP transport in MCP. It consists of a server that provides a simple time tool and a client that connects to it.

## Running the Example

1. First, start the server:
   ```bash
   go run server/main.go
   ```
   This will start an HTTP server on port 8080.

2. In another terminal, run the client:
   ```bash
   go run client/main.go
   ```

The client will:
1. Connect to the server
2. List available tools
3. Call the time tool with different time formats
4. Display the results

## Understanding the Code

- `server/main.go`: Shows how to create an MCP server using HTTP transport and register a tool
- `client/main.go`: Shows how to create an MCP client that connects to an HTTP server and calls tools

The example demonstrates:
- Setting up HTTP transport
- Tool registration and description
- Tool parameter handling
- Making tool calls with different arguments
- Handling tool responses 