package main

import (
	"context"
	"log"
	"time"

	"github.com/davecgh/go-spew/spew"
	mcp_golang "github.com/rvoh-emccaleb/mcp-golang"
	"github.com/rvoh-emccaleb/mcp-golang/transport/http"
)

const (
	ProtocolVersion = "2024-11-05"
)

func main() {
	// Create an HTTP transport that connects to the server
	transport := http.NewHTTPClientTransport("/mcp", 1*time.Millisecond)
	transport.WithBaseURL("http://localhost:8081")

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
			ProtocolVersion: ProtocolVersion,
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
}
