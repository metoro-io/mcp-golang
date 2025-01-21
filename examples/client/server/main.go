package main

import (
	"fmt"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

// HelloArgs represents the arguments for the hello tool
type HelloArgs struct {
	Name string `json:"name" jsonschema:"required,description=The name to say hello to"`
}

func main() {
	// Create a transport for the server
	serverTransport := stdio.NewStdioServerTransport()

	// Create a new server with the transport
	server := mcp.NewServer(serverTransport)

	// Register a simple tool with the server
	err := server.RegisterTool("hello", "Says hello", func(args HelloArgs) (*mcp.ToolResponse, error) {
		message := fmt.Sprintf("Hello, %s!", args.Name)
		return mcp.NewToolResponse(mcp.NewTextContent(message)), nil
	})
	if err != nil {
		panic(err)
	}

	// Start the server
	err = server.Serve()
	if err != nil {
		panic(err)
	}

	// Keep the server running
	select {}
}
