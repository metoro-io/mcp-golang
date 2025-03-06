<div align="center">
<img src="./resources/mcp-golang-logo.webp" height="300" alt="Statusphere logo">
</div>
<br/>
<div align="center">

![GitHub stars](https://img.shields.io/github/stars/metoro-io/mcp-golang?style=social)
![GitHub forks](https://img.shields.io/github/forks/metoro-io/mcp-golang?style=social)
![GitHub issues](https://img.shields.io/github/issues/metoro-io/mcp-golang)
![GitHub pull requests](https://img.shields.io/github/issues-pr/metoro-io/mcp-golang)
![GitHub license](https://img.shields.io/github/license/metoro-io/mcp-golang)
![GitHub contributors](https://img.shields.io/github/contributors/metoro-io/mcp-golang)
![GitHub last commit](https://img.shields.io/github/last-commit/metoro-io/mcp-golang)
[![GoDoc](https://pkg.go.dev/badge/github.com/metoro-io/mcp-golang.svg)](https://pkg.go.dev/github.com/metoro-io/mcp-golang)
[![Go Report Card](https://goreportcard.com/badge/github.com/metoro-io/mcp-golang)](https://goreportcard.com/report/github.com/metoro-io/mcp-golang)
![Tests](https://github.com/metoro-io/mcp-golang/actions/workflows/go-test.yml/badge.svg)




</div>

# mcp-golang 

mcp-golang is an unofficial implementation of the [Model Context Protocol](https://modelcontextprotocol.io/) in Go.

Write MCP servers and clients in golang with a few lines of code.

Docs at [https://mcpgolang.com](https://mcpgolang.com)

## Highlights
- 🛡️**Type safety** - Define your tool arguments as native go structs, have mcp-golang handle the rest. Automatic schema generation, deserialization, error handling etc.
- 🚛 **Custom transports** - Use the built-in transports (stdio for full feature support, HTTP for stateless communication) or write your own.
- ⚡ **Low boilerplate** - mcp-golang generates all the MCP endpoints for you apart from your tools, prompts and resources.
- 🧩 **Modular** - The library is split into three components: transport, protocol and server/client. Use them all or take what you need.
- 🔄 **Bi-directional** - Full support for both server and client implementations through stdio transport.

## Example Usage

Install with `go get github.com/metoro-io/mcp-golang`

### Server Example

```go
package main

import (
	"fmt"
	"github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

// Tool arguments are just structs, annotated with jsonschema tags
// More at https://mcpgolang.com/tools#schema-generation
type Content struct {
	Title       string  `json:"title" jsonschema:"required,description=The title to submit"`
	Description *string `json:"description" jsonschema:"description=The description to submit"`
}
type MyFunctionsArguments struct {
	Submitter string  `json:"submitter" jsonschema:"required,description=The name of the thing calling this tool (openai, google, claude, etc)"`
	Content   Content `json:"content" jsonschema:"required,description=The content of the message"`
}

func main() {
	done := make(chan struct{})

	server := mcp_golang.NewServer(stdio.NewStdioServerTransport())
	err := server.RegisterTool("hello", "Say hello to a person", func(arguments MyFunctionsArguments) (*mcp_golang.ToolResponse, error) {
		return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(fmt.Sprintf("Hello, %server!", arguments.Submitter))), nil
	})
	if err != nil {
		panic(err)
	}

	err = server.RegisterPrompt("promt_test", "This is a test prompt", func(arguments Content) (*mcp_golang.PromptResponse, error) {
		return mcp_golang.NewPromptResponse("description", mcp_golang.NewPromptMessage(mcp_golang.NewTextContent(fmt.Sprintf("Hello, %server!", arguments.Title)), mcp_golang.RoleUser)), nil
	})
	if err != nil {
		panic(err)
	}

	err = server.RegisterResource("test://resource", "resource_test", "This is a test resource", "application/json", func() (*mcp_golang.ResourceResponse, error) {
		return mcp_golang.NewResourceResponse(mcp_golang.NewTextEmbeddedResource("test://resource", "This is a test resource", "application/json")), nil
	})
	if err != nil {
		panic(err)
	}    

	err = server.Serve()
	if err != nil {
		panic(err)
	}

	<-done
}
```

### HTTP Server Example

You can also create an HTTP-based server using either the standard HTTP transport or Gin framework:

```go
// Standard HTTP
transport := http.NewHTTPTransport("/mcp")
transport.WithAddr(":8080")
server := mcp_golang.NewServer(transport)

// Or with Gin framework
transport := http.NewGinTransport()
router := gin.Default()
router.POST("/mcp", transport.Handler())
server := mcp_golang.NewServer(transport)
```

Note: HTTP transports are stateless and don't support bidirectional features like notifications. Use stdio transport if you need those features.

### Client Example

Checkout the [examples/client](./examples/client) directory for a more complete example.

```go
package main

import (
    "context"
    "log"
    mcp "github.com/metoro-io/mcp-golang"
    "github.com/metoro-io/mcp-golang/transport/stdio"
)

// Define type-safe arguments
type CalculateArgs struct {
    Operation string `json:"operation"`
    A         int    `json:"a"`
    B         int    `json:"b"`
}

func main() {
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
    // Create and initialize client
    transport := stdio.NewStdioServerTransportWithIO(stdout, stdin)
    client := mcp.NewClient(transport)
    
    if _, err := client.Initialize(context.Background()); err != nil {
        log.Fatalf("Failed to initialize: %v", err)
    }

    // Call a tool with typed arguments
    args := CalculateArgs{
        Operation: "add",
        A:         10,
        B:         5,
    }
    
    response, err := client.CallTool(context.Background(), "calculate", args)
    if err != nil {
        log.Fatalf("Failed to call tool: %v", err)
    }
    
    if response != nil && len(response.Content) > 0 {
        log.Printf("Result: %s", response.Content[0].TextContent.Text)
    }
}
```

### Using with Claude Desktop

Create a file in ~/Library/Application Support/Claude/claude_desktop_config.json with the following contents:

```json
{
"mcpServers": {
  "golang-mcp-server": {
      "command": "<your path to golang MCP server go executable>",
      "args": [],
      "env": {}
    }
  }
}
``` 

## Contributions

Contributions are more than welcome! Please check out [our contribution guidelines](./CONTRIBUTING.md).

## Discord

Got any suggestions, have a question on the api or usage? Ask on the [discord server](https://discord.gg/33saRwE3pT). 
A maintainer will be happy to help you out.

## Examples

Some more extensive examples using the library found here:

- <img height="12" width="12" src="https://metoro.io/static/images/logos/Metoro.svg" /> **[Metoro](https://github.com/metoro-io/metoro-mcp-server)** - Query and interact with kubernetes environments monitored by Metoro

Open a PR to add your own projects!

## Server Feature Implementation

### Tools
- [x] Tool Calls
- [x] Native go structs as arguments
- [x] Programatically generated tool list endpoint
- [x] Change notifications
- [x] Pagination

### Prompts
- [x] Prompt Calls
- [x] Programatically generated prompt list endpoint
- [x] Change notifications
- [x] Pagination

### Resources
- [x] Resource Calls
- [x] Programatically generated resource list endpoint
- [x] Change notifications
- [x] Pagination

### Transports
- [x] Stdio - Full support for all features including bidirectional communication
- [x] HTTP - Stateless transport for simple request-response scenarios (no notifications support)
- [x] Gin - HTTP transport with Gin framework integration (stateless, no notifications support)
- [x] SSE
- [x] Custom transport support
- [ ] HTTPS with custom auth support - in progress. Not currently part of the spec but we'll be adding experimental support for it.

### Client
- [x] Call tools
- [x] Call prompts
- [x] Call resources
- [x] List tools
- [x] List prompts
- [x] List resources

