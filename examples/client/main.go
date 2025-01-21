package main

import (
	"log"
	"os/exec"

	"context"

	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

func main() {
	// Start the server process
	cmd := exec.Command("go", "run", "./server/main.go")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatalf("Failed to get stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to get stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer cmd.Process.Kill()

	clientTransport := stdio.NewStdioServerTransportWithIO(stdout, stdin)
	client := mcp_golang.NewClient(clientTransport)

	if _, err := client.Initialize(context.Background()); err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
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

	// Example of calling the hello tool
	helloArgs := map[string]interface{}{
		"name": "World",
	}

	log.Println("\nCalling hello tool:")
	helloResponse, err := client.CallTool(context.Background(), "hello", helloArgs)
	if err != nil {
		log.Printf("Failed to call hello tool: %v", err)
	} else if helloResponse != nil && len(helloResponse.Content) > 0 && helloResponse.Content[0].TextContent != nil {
		log.Printf("Hello response: %s", helloResponse.Content[0].TextContent.Text)
	}

	// Example of calling the calculate tool
	calcArgs := map[string]interface{}{
		"operation": "add",
		"a":         10,
		"b":         5,
	}

	log.Println("\nCalling calculate tool:")
	calcResponse, err := client.CallTool(context.Background(), "calculate", calcArgs)
	if err != nil {
		log.Printf("Failed to call calculate tool: %v", err)
	} else if calcResponse != nil && len(calcResponse.Content) > 0 && calcResponse.Content[0].TextContent != nil {
		log.Printf("Calculate response: %s", calcResponse.Content[0].TextContent.Text)
	}

	// Example of calling the time tool
	timeArgs := map[string]interface{}{
		"format": "2006-01-02 15:04:05",
	}

	log.Println("\nCalling time tool:")
	timeResponse, err := client.CallTool(context.Background(), "time", timeArgs)
	if err != nil {
		log.Printf("Failed to call time tool: %v", err)
	} else if timeResponse != nil && len(timeResponse.Content) > 0 && timeResponse.Content[0].TextContent != nil {
		log.Printf("Time response: %s", timeResponse.Content[0].TextContent.Text)
	}

	// List available prompts
	prompts, err := client.ListPrompts(context.Background(), nil)
	if err != nil {
		log.Printf("Failed to list prompts: %v", err)
	} else {
		log.Println("\nAvailable Prompts:")
		for _, prompt := range prompts.Prompts {
			desc := ""
			if prompt.Description != nil {
				desc = *prompt.Description
			}
			log.Printf("Prompt: %s. Description: %s", prompt.Name, desc)
		}

		// Example of using the uppercase prompt
		promptArgs := map[string]interface{}{
			"input": "Hello, Model Context Protocol!",
		}

		log.Printf("\nCalling uppercase prompt:")
		upperResponse, err := client.GetPrompt(context.Background(), "uppercase", promptArgs)
		if err != nil {
			log.Printf("Failed to get uppercase prompt: %v", err)
		} else if upperResponse != nil && len(upperResponse.Messages) > 0 && upperResponse.Messages[0].Content != nil {
			log.Printf("Uppercase response: %s", upperResponse.Messages[0].Content.TextContent.Text)
		}

		// Example of using the reverse prompt
		log.Printf("\nCalling reverse prompt:")
		reverseResponse, err := client.GetPrompt(context.Background(), "reverse", promptArgs)
		if err != nil {
			log.Printf("Failed to get reverse prompt: %v", err)
		} else if reverseResponse != nil && len(reverseResponse.Messages) > 0 && reverseResponse.Messages[0].Content != nil {
			log.Printf("Reverse response: %s", reverseResponse.Messages[0].Content.TextContent.Text)
		}
	}
}
