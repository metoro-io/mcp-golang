package main

import (
	"log"
	"time"

	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/http"
)

// TimeArgs defines the arguments for the time tool
type TimeArgs struct {
	Format string `json:"format" jsonschema:"description=The time format to use"`
}

func main() {
	// Create an HTTP transport that listens on /mcp endpoint
	transport := http.NewHTTPTransport("/mcp").WithAddr(":8081")

	// Create a new server with the transport
	server := mcp_golang.NewServer(
		transport,
		mcp_golang.WithName("mcp-golang-stateless-http-example"),
		mcp_golang.WithInstructions("A simple example of a stateless HTTP server using mcp-golang"),
		mcp_golang.WithVersion("0.0.1"),
	)

	// Register a simple tool
	err := server.RegisterTool("time", "Returns the current time in the specified format", func(args TimeArgs) (*mcp_golang.ToolResponse, error) {
		format := args.Format
		return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(time.Now().Format(format))), nil
	})
	if err != nil {
		panic(err)
	}

	// Start the server
	log.Println("Starting HTTP server on :8081...")
	server.Serve()
}
