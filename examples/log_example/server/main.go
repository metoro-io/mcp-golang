package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

// HelloArgs represents the arguments for the hello tool
type HelloArgs struct {
	Name string `json:"name" jsonschema:"required,description=The name to say hello to"`
}

// CalculateArgs represents the arguments for the calculate tool
type CalculateArgs struct {
	Operation string  `json:"operation" jsonschema:"required,enum=add,enum=subtract,enum=multiply,enum=divide,description=The mathematical operation to perform"`
	A         float64 `json:"a" jsonschema:"required,description=First number"`
	B         float64 `json:"b" jsonschema:"required,description=Second number"`
}

// TimeArgs represents the arguments for the current time tool
type TimeArgs struct {
	Format string `json:"format,omitempty" jsonschema:"description=Optional time format (default: RFC3339)"`
}

// PromptArgs represents the arguments for custom prompts
type PromptArgs struct {
	Input string `json:"input" jsonschema:"required,description=The input text to process"`
}

func init() {
	logFile, _ := os.OpenFile("./server.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	log.SetOutput(logFile)
}

func main() {
	// Create a transport for the server
	serverTransport := stdio.NewStdioServerTransport()

	// Create a new server with the transport
	server := mcp.NewServer(serverTransport, mcp.WithLoggingCapability())

	// Register hello tool
	err := server.RegisterTool("hello", "Says hello to the provided name", func(args HelloArgs) (*mcp.ToolResponse, error) {
		message := fmt.Sprintf("Hello, %s!", args.Name)
		if err := server.SendLogMessageNotification(mcp.LevelDebug, "logger", message); err != nil {
			log.Printf("failed to send log message notification: %v\n", err)
			panic(err)
		}
		return mcp.NewToolResponse(mcp.NewTextContent(message)), nil
	})
	if err != nil {
		log.Printf("failed to register hello tool: %v\n", err)
		panic(err)
	}

	// Register calculate tool
	err = server.RegisterTool("calculate", "Performs basic mathematical operations", func(args CalculateArgs) (*mcp.ToolResponse, error) {
		var result float64
		switch args.Operation {
		case "add":
			result = args.A + args.B
		case "subtract":
			result = args.A - args.B
		case "multiply":
			result = args.A * args.B
		case "divide":
			if args.B == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			result = args.A / args.B
		default:
			return nil, fmt.Errorf("unknown operation: %s", args.Operation)
		}
		message := fmt.Sprintf("Result of %s: %.2f", args.Operation, result)
		if err := server.SendLogMessageNotification(mcp.LevelInfo, "logger", message); err != nil {
			log.Printf("failed to send log message notification: %v\n", err)
			panic(err)
		}
		return mcp.NewToolResponse(mcp.NewTextContent(message)), nil
	})
	if err != nil {
		panic(err)
	}

	// Register current time tool
	err = server.RegisterTool("time", "Returns the current time", func(args TimeArgs) (*mcp.ToolResponse, error) {
		format := time.RFC3339
		if args.Format != "" {
			format = args.Format
		}
		message := time.Now().Format(format)
		if err := server.SendLogMessageNotification(mcp.LevelNotice, "logger", message); err != nil {
			log.Printf("failed to send log message notification: %v\n", err)
			panic(err)
		}
		return mcp.NewToolResponse(mcp.NewTextContent(message)), nil
	})
	if err != nil {
		log.Printf("failed to register time tool: %v\n", err)
		panic(err)
	}

	// Register example prompts
	err = server.RegisterPrompt("uppercase", "Converts text to uppercase", func(args PromptArgs) (*mcp.PromptResponse, error) {
		text := strings.ToUpper(args.Input)
		return mcp.NewPromptResponse("uppercase", mcp.NewPromptMessage(mcp.NewTextContent(text), mcp.RoleUser)), nil
	})
	if err != nil {
		panic(err)
	}

	err = server.RegisterPrompt("reverse", "Reverses the input text", func(args PromptArgs) (*mcp.PromptResponse, error) {
		// Reverse the string
		runes := []rune(args.Input)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		text := string(runes)
		if err := server.SendLogMessageNotification(mcp.LevelWarning, "logger", text); err != nil {
			log.Printf("failed to send log message notification: %v\n", err)
			panic(err)
		}
		return mcp.NewPromptResponse("reverse", mcp.NewPromptMessage(mcp.NewTextContent(text), mcp.RoleUser)), nil
	})
	if err != nil {
		log.Printf("failed to register prompt: %v\n", err)
		panic(err)
	}

	// Start the server
	if err := server.Serve(); err != nil {
		log.Printf("failed to serve: %v\n", err)
		panic(err)
	}

	// Keep the server running
	select {}
}
