package mcp

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// Here we define the actual MCP server that users will create and run
// A server can be passed a number of handlers to handle requests from clients
// Additionally it can be parametrized by a transport. This transport will be used to actually send and receive messages.
// So for example if the stdio transport is used, the server will read from stdin and write to stdout
// If the SSE transport is used, the server will send messages over an SSE connection and receive messages from HTTP POST requests.

// The interface that we're looking to support is something like [gin](https://github.com/gin-gonic/gin)s interface
// Example use would be:

// func main() {
//     transport := mcp.NewStdioTransport()
//     server := mcp.NewServer(transport)
//

//       type HelloType struct {
//           Hello string `mcp:"description:'description',valudation:maxLength(10)`"`
//       }
//       type MyFunctionsArguments struct {
//           Foo string `mcp:"description:'description',validation:maxLength(10)`"`
//           Bar HelloType `mcp:"description:'description'`"`
//       }
//     server.Tool("test", "Test tool's description", MyFunctionsArguments{}, func(argument MyFunctionsArguments) (ToolResponse, error) {
//        let h := argument.Bar.Hello
//     })
//
//       arguments := NewObject(new Map[string]Argument]{
//		   "foo", NewString("description", NewStringValidation(Required, MaxLength(10))),
//		   "bar", NewObject(new Map[string]Argument]{
//		     "hello", NewStringValidation(Required, MaxLength(10))),
//		   }, NewObjectValidation(Required))
//		 )
//     server.Tool("test", "Test tool's description", arguments, func(argument Object) (ToolResponse, error) {
//        let bar, err := argument.GetString("bar")
//        if err != nil {
//            return nil, err
//        }
//        let h, err := bar.GetString("hello")
//		  if err != nil {
//			  return nil, err
//		  }
//     })
//
//
//     // Send a message
//     transport.Send(map[string]interface{}{
//         "jsonrpc": "2.0",
//         "method": "test",
//         "params": map[string]interface{}{},
//     })
// }

type Server struct {
	transport Transport
	tools     map[string]*ToolType
}

type ToolType struct {
	Name        string
	Description string
	Handler     func(interface{}) (ToolResponse, error)
	Arguments   interface{}
}

type ToolResponse struct {
	Result interface{} `json:"result"`
}

func NewServer(transport Transport) *Server {
	return &Server{
		transport: transport,
		tools:     make(map[string]*ToolType),
	}
}

// Tool registers a new tool with the server
func (s *Server) Tool(name string, description string, handler func(arguments interface{}) (ToolResponse, error)) {
	s.tools[name] = &ToolType{
		Name:        name,
		Description: description,
		Handler:     handler,
	}
}

// validateStruct validates a struct based on its mcp tags
func validateStruct(v interface{}) error {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("mcp")
		if tag == "" {
			continue
		}

		// Parse the tag
		tagMap := parseTag(tag)

		// Get validation rules
		if validation, ok := tagMap["validation"]; ok {
			if strings.Contains(validation, "maxLength") {
				length := extractMaxLength(validation)
				fieldVal := val.Field(i)
				if fieldVal.Kind() == reflect.String && fieldVal.Len() > length {
					return fmt.Errorf("field %s exceeds maximum length of %d", field.Name, length)
				}
			}
		}

		// If it's a struct, recursively validate
		if field.Type.Kind() == reflect.Struct {
			if err := validateStruct(val.Field(i).Interface()); err != nil {
				return err
			}
		}
	}
	return nil
}

// parseTag parses an mcp tag into a map of key-value pairs
func parseTag(tag string) map[string]string {
	result := make(map[string]string)
	parts := strings.Split(tag, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) == 2 {
			result[strings.TrimSpace(kv[0])] = strings.Trim(kv[1], "'")
		}
	}
	return result
}

// extractMaxLength extracts the maximum length from a maxLength validation rule
func extractMaxLength(validation string) int {
	re := regexp.MustCompile(`maxLength\((\d+)\)`)
	matches := re.FindStringSubmatch(validation)
	if len(matches) == 2 {
		length, _ := strconv.Atoi(matches[1])
		return length
	}
	return 0
}

type HelloType struct {
	Hello string `mcp:"description:'description',validation:maxLength(10)"`
}
type MyFunctionsArguments struct {
	Foo string    `mcp:"description:'description',validation:maxLength(10)"`
	Bar HelloType `mcp:"description:'description'"`
}

func main() {
	s := NewServer(NewStdioServerTransport())
	s.Tool("test", "Test tool's description", func(arguments MyFunctionsArguments) (ToolResponse, error) {
		h := arguments.Bar.Hello
		// ... handle the tool logic
		return ToolResponse{Result: h}, nil
	})
}