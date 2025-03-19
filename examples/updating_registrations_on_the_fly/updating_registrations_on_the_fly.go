package main

import (
	"fmt"
	mcp_golang "github.com/rvoh-emccaleb/mcp-golang"
	"github.com/rvoh-emccaleb/mcp-golang/transport/stdio"
	"time"
)

type HelloArguments struct {
	Submitter string `json:"submitter" jsonschema:"required,description=The name of the thing calling this tool (openai or google or claude etc)'"`
}

type Content struct {
	Title       string  `json:"title" jsonschema:"required,description=The title to submit"`
	Description *string `json:"description" jsonschema:"description=The description to submit"`
}

// This is a stupid server that demonstrates how to update registrations on the fly.
// Every second the server will register a new tool, a new prompt and a new resource, then unregister the old ones.
func main() {
	done := make(chan struct{})
	server := mcp_golang.NewServer(stdio.NewStdioServerTransport())
	err := server.Serve()
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			err := server.RegisterTool("hello", "Say hello to a person", func(arguments HelloArguments) (*mcp_golang.ToolResponse, error) {
				return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Hello, %s!", arguments.Submitter))), nil
			})
			if err != nil {
				panic(err)
			}
			time.Sleep(1 * time.Second)
			err = server.DeregisterTool("hello")
			if err != nil {
				panic(err)
			}
		}
	}()
	go func() {
		for {

			err = server.RegisterPrompt("prompt_test", "This is a test prompt", func(arguments Content) (*mcp_golang.PromptResponse, error) {
				return mcp_golang.NewPromptResponse("description", mcp_golang.NewPromptMessage(mcp_golang.NewTextContent(fmt.Sprintf("Hello, %server!", arguments.Title)), mcp_golang.RoleUser)), nil
			})
			if err != nil {
				panic(err)
			}
			time.Sleep(1 * time.Second)
			err = server.DeregisterPrompt("prompt_test")
			if err != nil {
				panic(err)
			}
		}

	}()
	go func() {
		err = server.RegisterResource("test://resource", "resource_test", "This is a test resource", "application/json", func() (*mcp_golang.ResourceResponse, error) {
			return mcp_golang.NewResourceResponse(mcp_golang.NewTextEmbeddedResource("test://resource", "This is a test resource", "application/json")), nil
		})
		if err != nil {
			panic(err)
		}
		time.Sleep(1 * time.Second)
		err = server.DeregisterResource("test://resource")
		if err != nil {
			panic(err)
		}
	}()

	<-done
}
