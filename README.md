<div align="center">
<img src="./resources/mcp-golang-logo.webp" height="300" alt="Statusphere logo">
</div>
<br/>
<div align="center" style="font-size:46px;">
<b>MCP Golang</b>
</div>

<div align="center">

![GitHub stars](https://img.shields.io/github/stars/metoro-io/mcp-golang?style=social)
![GitHub forks](https://img.shields.io/github/forks/metoro-io/mcp-golang?style=social)
![GitHub issues](https://img.shields.io/github/issues/metoro-io/mcp-golang)
![GitHub pull requests](https://img.shields.io/github/issues-pr/metoro-io/mcp-golang)
![GitHub license](https://img.shields.io/github/license/metoro-io/mcp-golang)
![GitHub contributors](https://img.shields.io/github/contributors/metoro-io/mcp-golang)
![GitHub last commit](https://img.shields.io/github/last-commit/metoro-io/mcp-golang)

</div>

MCP golang is an unofficial implementaion of the [Model Context Protocol](https://modelcontextprotocol.io/) in Go.

Docs can be found at [https://mcpgolang.com](https://mcpgolang.com)

## Example Usage

```go
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
    s := server.NewServer(stdio.NewStdioServerTransport())
    err := s.RegisterTool("hello", "Say hello to a person", func (arguments MyFunctionsArguments) (*server.ToolResponse, error) {
        return server.NewToolReponse(server.NewTextContent(fmt.Sprintf("Hello, %s!", arguments.Submitter))), nil
    })
    if err != nil {
        panic(err)
    }
    <-done
}
```

This will start a server using the stdio transport (used by claude desktop), host a tool called "hello" that will say hello to the user who submitted it.

You can use raw go structs as the input to your tools, the library handles generating the messages, deserialization, etc.


## Library Goals and non-goals

- The libary api should be simple and easy to use for basic cases of the protocol but we are explicitly aiming to support production use cases firs
- Where complexity arises, the library will have sane defaults and allow the user to override them if needed.
- The library aims to support servers first and foremost, when the server features are stable, we will work on adding client support.

## Contributions

Contributions are more than welcome! Please check out [our contribution guidelines](./CONTRIBUTING.md).

## Discord

Got any suggestions, have a question on the api or usage? Ask on the [discord server](https://discord.gg/33saRwE3pT). 
A maintainer will be happy to help you out.

## Server Feature Implementation

### Tools
- [x] Tool Calls
- [x] Programatically generated tool list endpoint

### Prompts
- [x] Prompt Calls
- [x] Programatically generated prompt list endpoint

### Resources
- [x] Resource Calls
- [x] Programatically generated resource list endpoint

### Transports
- [x] Stdio
- [x] SSE
- [x] Custom transport support
- [ ] HTTPS with custom auth support - in progress. Not currently part of the spec but we'll be adding experimental support for it.

## Client Feature Implementation

Currently under development
