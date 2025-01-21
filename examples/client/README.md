# MCP Client Example

This example demonstrates how to use the Model Context Protocol (MCP) client to interact with an MCP server. The example includes both a client and a server implementation, showcasing various MCP features like tools and prompts.

## Features Demonstrated

- Client initialization and connection to server
- Listing available tools
- Calling different tools:
  - Hello tool: Basic greeting functionality
  - Calculate tool: Simple arithmetic operations
  - Time tool: Current time formatting
- Listing available prompts
- Using prompts:
  - Uppercase prompt: Converts text to uppercase
  - Reverse prompt: Reverses input text

## Running the Example

1. Make sure you're in the `examples/client` directory:
   ```bash
   cd examples/client
   ```

2. Run the example:
   ```bash
   go run main.go
   ```

The program will:
1. Start a local MCP server (implemented in `server/main.go`)
2. Create an MCP client and connect to the server
3. Demonstrate various interactions with the server

## Expected Output

You should see output similar to this:

```
Available Tools:
Tool: hello. Description: A simple greeting tool
Tool: calculate. Description: A basic calculator
Tool: time. Description: Returns formatted current time

Calling hello tool:
Hello response: Hello, World!

Calling calculate tool:
Calculate response: Result of 10 + 5 = 15

Calling time tool:
Time response: [current time in format: 2006-01-02 15:04:05]

Available Prompts:
Prompt: uppercase. Description: Converts text to uppercase
Prompt: reverse. Description: Reverses the input text

Calling uppercase prompt:
Uppercase response: HELLO, MODEL CONTEXT PROTOCOL!

Calling reverse prompt:
Reverse response: !locotorP txetnoC ledoM ,olleH
```

## Code Structure

- `main.go`: Client implementation and example usage
- `server/main.go`: Example MCP server implementation with sample tools and prompts

## Notes

- The server is automatically started and stopped by the client program
- The example uses stdio transport for communication between client and server
- All tools and prompts are simple examples to demonstrate the protocol functionality 