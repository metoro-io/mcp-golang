package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

// HelloArgs represents the arguments for the hello tool
type HelloArgs struct {
	Name string `json:"name" jsonschema:"required,description=The name to say hello to"`
}

// runServer starts an MCP server that registers a hello tool
func runServer() {
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

// runClient starts an MCP client that connects to the server and interacts with it
func runClient() {
	// Start the server process
	cmd := exec.Command("go", "run", "-v", "-")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatalf("Failed to get stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to get stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatalf("Failed to get stderr pipe: %v", err)
	}

	// Pass the current source code to the server process
	cmd.Env = append(cmd.Env, "RUN_AS_SERVER=true")

	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer cmd.Process.Kill()

	// Start a goroutine to read stderr
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("Error reading stderr: %v", err)
				}
				return
			}
			if n > 0 {
				log.Printf("Server stderr: %s", string(buf[:n]))
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
			Id:      transport.RequestId(1),
		}

		reqBytes, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}
		reqBytes = append(reqBytes, '\n')

		log.Printf("Sending request: %s", string(reqBytes))
		_, err = stdin.Write(reqBytes)
		if err != nil {
			return nil, err
		}

		// Read response with timeout
		respChan := make(chan map[string]interface{}, 1)
		errChan := make(chan error, 1)

		go func() {
			decoder := json.NewDecoder(stdout)
			var response map[string]interface{}
			err := decoder.Decode(&response)
			if err != nil {
				errChan <- fmt.Errorf("failed to decode response: %v", err)
				return
			}
			log.Printf("Got response: %+v", response)
			respChan <- response
		}()

		select {
		case resp := <-respChan:
			return resp, nil
		case err := <-errChan:
			return nil, err
		case <-time.After(5 * time.Second):
			return nil, fmt.Errorf("timeout waiting for response")
		}
	}

	// Initialize the server
	resp, err := sendRequest("initialize", map[string]interface{}{
		"capabilities": map[string]interface{}{},
	})
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}
	log.Printf("Server initialized: %+v", resp)

	// List tools
	resp, err = sendRequest("tools/list", map[string]interface{}{})
	if err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}

	// Print the available tools
	fmt.Println("\nAvailable tools:")
	tools := resp["result"].(map[string]interface{})["tools"].([]interface{})
	for _, t := range tools {
		tool := t.(map[string]interface{})
		fmt.Printf("- %s: %s\n", tool["name"], tool["description"])
	}

	// Call the hello tool
	callParams := map[string]interface{}{
		"name": "hello",
		"arguments": map[string]interface{}{
			"name": "World",
		},
	}
	resp, err = sendRequest("tools/call", callParams)
	if err != nil {
		log.Fatalf("Failed to call hello tool: %v", err)
	}

	// Print the response
	fmt.Println("\nTool response:")
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})[0].(map[string]interface{})
	fmt.Println(content["text"])
}

func main() {
	// If RUN_AS_SERVER is set, run as server, otherwise run as client
	if os.Getenv("RUN_AS_SERVER") == "true" {
		runServer()
	} else {
		runClient()
	}
}
