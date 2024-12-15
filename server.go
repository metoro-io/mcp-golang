package mcp_golang

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/invopop/jsonschema"
	"github.com/metoro-io/mcp-golang/internal/datastructures"
	"github.com/metoro-io/mcp-golang/internal/protocol"
	"github.com/metoro-io/mcp-golang/internal/tools"
	"github.com/metoro-io/mcp-golang/transport"
	"reflect"
	"sort"
	"strings"
)

// Here we define the actual MCP server that users will create and run
// A server can be passed a number of handlers to handle requests from clients
// Additionally it can be parametrized by a transport. This transport will be used to actually send and receive messages.
// So for example if the stdio transport is used, the server will read from stdin and write to stdout
// If the SSE transport is used, the server will send messages over an SSE connection and receive messages from HTTP POST requests.

// The interface that we're looking to support is something like [gin](https://github.com/gin-gonic/gin)s interface

type toolResponseSent struct {
	Response *ToolResponse
	Error    error
}

// Custom JSON marshaling for ToolResponse
func (c toolResponseSent) MarshalJSON() ([]byte, error) {
	if c.Error != nil {
		errorText := c.Error.Error()
		c.Response = NewToolResponse(NewTextContent(errorText))
	}
	return json.Marshal(struct {
		Content []*Content `json:"content" yaml:"content" mapstructure:"content"`
		IsError bool       `json:"isError" yaml:"isError" mapstructure:"isError"`
	}{
		Content: c.Response.Content,
		IsError: c.Error != nil,
	})
}

// Custom JSON marshaling for ToolResponse
func (c resourceResponseSent) MarshalJSON() ([]byte, error) {
	if c.Error != nil {
		errorText := c.Error.Error()
		c.Response = NewResourceResponse(NewTextEmbeddedResource(c.Uri, errorText, "text/plain"))
	}
	return json.Marshal(c.Response)
}

type resourceResponseSent struct {
	Response *ResourceResponse
	Uri      string
	Error    error
}

func newResourceResponseSentError(err error) *resourceResponseSent {
	return &resourceResponseSent{
		Error: err,
	}
}

// newToolResponseSent creates a new toolResponseSent
func newResourceResponseSent(response *ResourceResponse) *resourceResponseSent {
	return &resourceResponseSent{
		Response: response,
	}
}

type promptResponseSent struct {
	Response *PromptResponse
	Error    error
}

func newPromptResponseSentError(err error) *promptResponseSent {
	return &promptResponseSent{
		Error: err,
	}
}

// newToolResponseSent creates a new toolResponseSent
func newPromptResponseSent(response *PromptResponse) *promptResponseSent {
	return &promptResponseSent{
		Response: response,
	}
}

// Custom JSON marshaling for PromptResponse
func (c promptResponseSent) MarshalJSON() ([]byte, error) {
	if c.Error != nil {
		errorText := c.Error.Error()
		c.Response = NewPromptResponse("error", NewPromptMessage(NewTextContent(errorText), RoleUser))
	}
	return json.Marshal(c.Response)
}

type Server struct {
	isRunning          bool
	transport          transport.Transport
	protocol           *protocol.Protocol
	paginationLimit    *int
	tools              *datastructures.SyncMap[string, *tool]
	prompts            *datastructures.SyncMap[string, *prompt]
	resources          *datastructures.SyncMap[string, *resource]
	serverInstructions *string
	serverName         string
	serverVersion      string
}

type prompt struct {
	Name              string
	Description       string
	Handler           func(baseGetPromptRequestParamsArguments) *promptResponseSent
	PromptInputSchema *promptSchema
}

type tool struct {
	Name            string
	Description     string
	Handler         func(baseCallToolRequestParams) *toolResponseSent
	ToolInputSchema *jsonschema.Schema
}

type resource struct {
	Name        string
	Description string
	Uri         string
	mimeType    string
	Handler     func() *resourceResponseSent
}

type ServerOptions func(*Server)

func WithProtocol(protocol *protocol.Protocol) ServerOptions {
	return func(s *Server) {
		s.protocol = protocol
	}
}

// Beware: As of 2024-12-13, it looks like Claude does not support pagination yet
func WithPaginationLimit(limit int) ServerOptions {
	return func(s *Server) {
		s.paginationLimit = &limit
	}
}

func NewServer(transport transport.Transport, options ...ServerOptions) *Server {
	server := &Server{
		protocol:  protocol.NewProtocol(nil),
		transport: transport,
		tools:     new(datastructures.SyncMap[string, *tool]),
		prompts:   new(datastructures.SyncMap[string, *prompt]),
		resources: new(datastructures.SyncMap[string, *resource]),
	}
	for _, option := range options {
		option(server)
	}
	return server
}

// RegisterTool registers a new tool with the server
func (s *Server) RegisterTool(name string, description string, handler any) error {
	err := validateToolHandler(handler)
	if err != nil {
		return err
	}
	inputSchema := createJsonSchemaFromHandler(handler)

	s.tools.Store(name, &tool{
		Name:            name,
		Description:     description,
		Handler:         createWrappedToolHandler(handler),
		ToolInputSchema: inputSchema,
	})

	return s.sendToolListChangedNotification()
}

func (s *Server) sendToolListChangedNotification() error {
	if !s.isRunning {
		return nil
	}
	return s.protocol.Notification("notifications/tools/list_changed", nil)
}

func (s *Server) CheckToolRegistered(name string) bool {
	_, ok := s.tools.Load(name)
	return ok
}

func (s *Server) DeregisterTool(name string) error {
	s.tools.Delete(name)
	return s.sendToolListChangedNotification()
}

func (s *Server) RegisterResource(uri string, name string, description string, mimeType string, handler any) error {
	err := validateResourceHandler(handler)
	if err != nil {
		panic(err)
	}
	s.resources.Store(uri, &resource{
		Name:        name,
		Description: description,
		Uri:         uri,
		mimeType:    mimeType,
		Handler:     createWrappedResourceHandler(handler),
	})
	return s.sendResourceListChangedNotification()
}

func (s *Server) sendResourceListChangedNotification() error {
	if !s.isRunning {
		return nil
	}
	return s.protocol.Notification("notifications/resources/list_changed", nil)
}

func (s *Server) CheckResourceRegistered(uri string) bool {
	_, ok := s.resources.Load(uri)
	return ok
}

func (s *Server) DeregisterResource(uri string) error {
	s.resources.Delete(uri)
	return s.sendResourceListChangedNotification()
}

func createWrappedResourceHandler(userHandler any) func() *resourceResponseSent {
	handlerValue := reflect.ValueOf(userHandler)
	return func() *resourceResponseSent {
		// Call the handler with no arguments
		output := handlerValue.Call([]reflect.Value{})

		if len(output) != 2 {
			return newResourceResponseSentError(fmt.Errorf("handler must return exactly two values, got %d", len(output)))
		}

		if !output[0].CanInterface() {
			return newResourceResponseSentError(fmt.Errorf("handler must return a struct, got %s", output[0].Type().Name()))
		}
		promptR := output[0].Interface()
		if !output[1].CanInterface() {
			return newResourceResponseSentError(fmt.Errorf("handler must return an error, got %s", output[1].Type().Name()))
		}
		errorOut := output[1].Interface()
		if errorOut == nil {
			return newResourceResponseSent(promptR.(*ResourceResponse))
		}
		return newResourceResponseSentError(errorOut.(error))
	}
}

// We just want to check that handler takes no arguments and returns a ResourceResponse and an error
func validateResourceHandler(handler any) error {
	handlerValue := reflect.ValueOf(handler)
	handlerType := handlerValue.Type()
	if handlerType.NumIn() != 0 {
		return fmt.Errorf("handler must take no arguments, got %d", handlerType.NumIn())
	}
	if handlerType.NumOut() != 2 {
		return fmt.Errorf("handler must return exactly two values, got %d", handlerType.NumOut())
	}
	//if handlerType.Out(0) != reflect.TypeOf((*ResourceResponse)(nil)).Elem() {
	//	return fmt.Errorf("handler must return ResourceResponse, got %s", handlerType.Out(0).Name())
	//}
	//if handlerType.Out(1) != reflect.TypeOf((*error)(nil)).Elem() {
	//	return fmt.Errorf("handler must return error, got %s", handlerType.Out(1).Name())
	//}
	return nil
}

func (s *Server) RegisterPrompt(name string, description string, handler any) error {
	err := validatePromptHandler(handler)
	if err != nil {
		return err
	}
	promptSchema := createPromptSchemaFromHandler(handler)
	s.prompts.Store(name, &prompt{
		Name:              name,
		Description:       description,
		Handler:           createWrappedPromptHandler(handler),
		PromptInputSchema: promptSchema,
	})

	return s.sendPromptListChangedNotification()
}

func (s *Server) sendPromptListChangedNotification() error {
	if !s.isRunning {
		return nil
	}
	return s.protocol.Notification("notifications/prompts/list_changed", nil)
}

func (s *Server) CheckPromptRegistered(name string) bool {
	_, ok := s.prompts.Load(name)
	return ok
}

func (s *Server) DeregisterPrompt(name string) error {
	s.prompts.Delete(name)
	return s.sendPromptListChangedNotification()
}

func createWrappedPromptHandler(userHandler any) func(baseGetPromptRequestParamsArguments) *promptResponseSent {
	handlerValue := reflect.ValueOf(userHandler)
	handlerType := handlerValue.Type()
	argumentType := handlerType.In(0)
	return func(arguments baseGetPromptRequestParamsArguments) *promptResponseSent {
		// Instantiate a struct of the type of the arguments
		if !reflect.New(argumentType).CanInterface() {
			return newPromptResponseSentError(fmt.Errorf("arguments must be a struct"))
		}
		unmarshaledArguments := reflect.New(argumentType).Interface()

		// Unmarshal the JSON into the correct type
		err := json.Unmarshal(arguments.Arguments, &unmarshaledArguments)
		if err != nil {
			return newPromptResponseSentError(fmt.Errorf("failed to unmarshal arguments: %w", err))
		}

		// Need to dereference the unmarshaled arguments
		of := reflect.ValueOf(unmarshaledArguments)
		if of.Kind() != reflect.Ptr || !of.Elem().CanInterface() {
			return newPromptResponseSentError(fmt.Errorf("arguments must be a struct"))
		}
		// Call the handler with the typed arguments
		output := handlerValue.Call([]reflect.Value{of.Elem()})

		if len(output) != 2 {
			return newPromptResponseSentError(fmt.Errorf("handler must return exactly two values, got %d", len(output)))
		}

		if !output[0].CanInterface() {
			return newPromptResponseSentError(fmt.Errorf("handler must return a struct, got %s", output[0].Type().Name()))
		}
		promptR := output[0].Interface()
		if !output[1].CanInterface() {
			return newPromptResponseSentError(fmt.Errorf("handler must return an error, got %s", output[1].Type().Name()))
		}
		errorOut := output[1].Interface()
		if errorOut == nil {
			return newPromptResponseSent(promptR.(*PromptResponse))
		}
		return newPromptResponseSentError(errorOut.(error))
	}
}

// Get the argument and iterate over the fields, we pull description from the jsonschema description tag
// We pull required from the jsonschema required tag
// Example:
// type Content struct {
// Title       string  `json:"title" jsonschema:"description=The title to submit,required"`
// Description *string `json:"description" jsonschema:"description=The description to submit"`
// }
// Then we get the jsonschema for the struct where Title is a required field and Description is an optional field
func createPromptSchemaFromHandler(handler any) *promptSchema {
	handlerValue := reflect.ValueOf(handler)
	handlerType := handlerValue.Type()
	argumentType := handlerType.In(0)

	promptSchema := promptSchema{
		Arguments: make([]promptSchemaArgument, argumentType.NumField()),
	}

	for i := 0; i < argumentType.NumField(); i++ {
		field := argumentType.Field(i)
		fieldName := field.Name

		jsonSchemaTags := strings.Split(field.Tag.Get("jsonschema"), ",")
		var description *string
		var required = false
		for _, tag := range jsonSchemaTags {
			if strings.HasPrefix(tag, "description=") {
				s := strings.TrimPrefix(tag, "description=")
				description = &s
			}
			if tag == "required" {
				required = true
			}
		}

		promptSchema.Arguments[i] = promptSchemaArgument{
			Name:        fieldName,
			Description: description,
			Required:    &required,
		}
	}
	return &promptSchema
}

// A prompt can only take a struct with fields of type string or *string as the argument
func validatePromptHandler(handler any) error {
	handlerValue := reflect.ValueOf(handler)
	handlerType := handlerValue.Type()
	argumentType := handlerType.In(0)

	if argumentType.Kind() != reflect.Struct {
		return fmt.Errorf("argument must be a struct")
	}

	for i := 0; i < argumentType.NumField(); i++ {
		field := argumentType.Field(i)
		isValid := false
		if field.Type.Kind() == reflect.String {
			isValid = true
		}
		if field.Type.Kind() == reflect.Ptr && field.Type.Elem().Kind() == reflect.String {
			isValid = true
		}
		if !isValid {
			return fmt.Errorf("all fields of the struct must be of type string or *string, found %s", field.Type.Kind())
		}
	}
	return nil
}

// Creates a full JSON schema from a user provided handler by introspecting the arguments
func createJsonSchemaFromHandler(handler any) *jsonschema.Schema {
	handlerValue := reflect.ValueOf(handler)
	handlerType := handlerValue.Type()
	argumentType := handlerType.In(0)
	inputSchema := jsonSchemaReflector.ReflectFromType(argumentType)
	return inputSchema
}

// This takes a user provided handler and returns a wrapped handler which can be used to actually answer requests
// Concretely, it will deserialize the arguments and call the user provided handler and then serialize the response
// If the handler returns an error, it will be serialized and sent back as a tool error rather than a protocol error
func createWrappedToolHandler(userHandler any) func(baseCallToolRequestParams) *toolResponseSent {
	handlerValue := reflect.ValueOf(userHandler)
	handlerType := handlerValue.Type()
	argumentType := handlerType.In(0)
	return func(arguments baseCallToolRequestParams) *toolResponseSent {
		// Instantiate a struct of the type of the arguments
		if !reflect.New(argumentType).CanInterface() {
			return newToolResponseSentError(fmt.Errorf("arguments must be a struct"))
		}
		unmarshaledArguments := reflect.New(argumentType).Interface()

		// Unmarshal the JSON into the correct type
		err := json.Unmarshal(arguments.Arguments, &unmarshaledArguments)
		if err != nil {
			return newToolResponseSentError(fmt.Errorf("failed to unmarshal arguments: %w", err))
		}

		// Need to dereference the unmarshaled arguments
		of := reflect.ValueOf(unmarshaledArguments)
		if of.Kind() != reflect.Ptr || !of.Elem().CanInterface() {
			return newToolResponseSentError(fmt.Errorf("arguments must be a struct"))
		}
		// Call the handler with the typed arguments
		output := handlerValue.Call([]reflect.Value{of.Elem()})

		if len(output) != 2 {
			return newToolResponseSentError(fmt.Errorf("handler must return exactly two values, got %d", len(output)))
		}

		if !output[0].CanInterface() {
			return newToolResponseSentError(fmt.Errorf("handler must return a struct, got %s", output[0].Type().Name()))
		}
		tool := output[0].Interface()
		if !output[1].CanInterface() {
			return newToolResponseSentError(fmt.Errorf("handler must return an error, got %s", output[1].Type().Name()))
		}
		errorOut := output[1].Interface()
		if errorOut == nil {
			return newToolResponseSent(tool.(*ToolResponse))
		}
		return newToolResponseSentError(errorOut.(error))
	}
}

func (s *Server) Serve() error {
	if s.isRunning == true {
		return fmt.Errorf("server is already running")
	}
	pr := s.protocol
	pr.SetRequestHandler("ping", s.handlePing)
	pr.SetRequestHandler("initialize", s.handleInitialize)
	pr.SetRequestHandler("tools/list", s.handleListTools)
	pr.SetRequestHandler("tools/call", s.handleToolCalls)
	pr.SetRequestHandler("prompts/list", s.handleListPrompts)
	pr.SetRequestHandler("prompts/get", s.handlePromptCalls)
	pr.SetRequestHandler("resources/list", s.handleListResources)
	pr.SetRequestHandler("resources/read", s.handleResourceCalls)
	err := pr.Connect(s.transport)
	if err != nil {
		return err
	}
	s.protocol = pr
	s.isRunning = true
	return nil
}

func (s *Server) handleInitialize(_ *transport.BaseJSONRPCRequest, _ protocol.RequestHandlerExtra) (transport.JsonRpcBody, error) {
	return initializeResult{
		Meta:            nil,
		Capabilities:    s.generateCapabilities(),
		Instructions:    s.serverInstructions,
		ProtocolVersion: "2024-11-05",
		ServerInfo: implementation{
			Name:    s.serverName,
			Version: s.serverVersion,
		},
	}, nil
}

func (s *Server) handleListTools(request *transport.BaseJSONRPCRequest, _ protocol.RequestHandlerExtra) (transport.JsonRpcBody, error) {
	type toolRequestParams struct {
		Cursor *string `json:"cursor"`
	}
	var params toolRequestParams
	err := json.Unmarshal(request.Params, &params)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	// Order by name for pagination
	var orderedTools []*tool
	s.tools.Range(func(k string, t *tool) bool {
		orderedTools = append(orderedTools, t)
		return true
	})
	sort.Slice(orderedTools, func(i, j int) bool {
		return orderedTools[i].Name < orderedTools[j].Name
	})

	startPosition := 0
	if params.Cursor != nil {
		// Base64 decode the cursor
		c, err := base64.StdEncoding.DecodeString(*params.Cursor)
		if err != nil {
			return nil, fmt.Errorf("failed to decode cursor: %w", err)
		}
		cString := string(c)
		// Iterate through the tools until we find an entry > the cursor
		found := false
		for i := 0; i < len(orderedTools); i++ {
			if orderedTools[i].Name > cString {
				startPosition = i
				found = true
				break
			}
		}
		if !found {
			startPosition = len(orderedTools)
		}
	}
	endPosition := len(orderedTools)
	if s.paginationLimit != nil {
		// Make sure we don't go out of bounds
		if len(orderedTools) > startPosition+*s.paginationLimit {
			endPosition = startPosition + *s.paginationLimit
		}
	}

	toolsToReturn := make([]tools.ToolRetType, 0)

	for i := startPosition; i < endPosition; i++ {
		toolsToReturn = append(toolsToReturn, tools.ToolRetType{
			Name:        orderedTools[i].Name,
			Description: &orderedTools[i].Description,
			InputSchema: orderedTools[i].ToolInputSchema,
		})
	}

	return tools.ToolsResponse{
		Tools: toolsToReturn,
		NextCursor: func() *string {
			if s.paginationLimit != nil && len(toolsToReturn) >= *s.paginationLimit {
				toString := base64.StdEncoding.EncodeToString([]byte(toolsToReturn[len(toolsToReturn)-1].Name))
				return &toString
			}
			return nil
		}(),
	}, nil
}

func (s *Server) handleToolCalls(req *transport.BaseJSONRPCRequest, _ protocol.RequestHandlerExtra) (transport.JsonRpcBody, error) {
	params := baseCallToolRequestParams{}
	// Instantiate a struct of the type of the arguments
	err := json.Unmarshal(req.Params, &params)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	var toolToUse *tool
	s.tools.Range(func(k string, t *tool) bool {
		if k != params.Name {
			return true
		}
		toolToUse = t
		return false
	})

	if toolToUse == nil {
		return nil, fmt.Errorf("unknown tool: %s", req.Method)
	}
	return toolToUse.Handler(params), nil
}

func (s *Server) generateCapabilities() serverCapabilities {
	t := false
	return serverCapabilities{
		Tools: func() *serverCapabilitiesTools {
			return &serverCapabilitiesTools{
				ListChanged: &t,
			}
		}(),
		Prompts: func() *serverCapabilitiesPrompts {
			return &serverCapabilitiesPrompts{
				ListChanged: &t,
			}
		}(),
		Resources: func() *serverCapabilitiesResources {
			return &serverCapabilitiesResources{
				ListChanged: &t,
			}
		}(),
	}
}

func (s *Server) handleListPrompts(request *transport.BaseJSONRPCRequest, extra protocol.RequestHandlerExtra) (transport.JsonRpcBody, error) {
	type promptRequestParams struct {
		Cursor *string `json:"cursor"`
	}
	var params promptRequestParams
	err := json.Unmarshal(request.Params, &params)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	// Order by name for pagination
	var orderedPrompts []*prompt
	s.prompts.Range(func(k string, p *prompt) bool {
		orderedPrompts = append(orderedPrompts, p)
		return true
	})
	sort.Slice(orderedPrompts, func(i, j int) bool {
		return orderedPrompts[i].Name < orderedPrompts[j].Name
	})

	startPosition := 0
	if params.Cursor != nil {
		// Base64 decode the cursor
		c, err := base64.StdEncoding.DecodeString(*params.Cursor)
		if err != nil {
			return nil, fmt.Errorf("failed to decode cursor: %w", err)
		}
		cString := string(c)
		// Iterate through the prompts until we find an entry > the cursor
		for i := 0; i < len(orderedPrompts); i++ {
			if orderedPrompts[i].Name > cString {
				startPosition = i
				break
			}
		}
	}
	endPosition := len(orderedPrompts)
	if s.paginationLimit != nil {
		// Make sure we don't go out of bounds
		if len(orderedPrompts) > startPosition+*s.paginationLimit {
			endPosition = startPosition + *s.paginationLimit
		}
	}

	promptsToReturn := make([]*promptSchema, 0)
	for i := startPosition; i < endPosition; i++ {
		schema := orderedPrompts[i].PromptInputSchema
		schema.Name = orderedPrompts[i].Name
		promptsToReturn = append(promptsToReturn, schema)
	}

	return listPromptsResult{
		Prompts: promptsToReturn,
		NextCursor: func() *string {
			if s.paginationLimit != nil && len(promptsToReturn) >= *s.paginationLimit {
				toString := base64.StdEncoding.EncodeToString([]byte(promptsToReturn[len(promptsToReturn)-1].Name))
				return &toString
			}
			return nil
		}(),
	}, nil
}

func (s *Server) handleListResources(request *transport.BaseJSONRPCRequest, extra protocol.RequestHandlerExtra) (transport.JsonRpcBody, error) {
	type resourceRequestParams struct {
		Cursor *string `json:"cursor"`
	}
	var params resourceRequestParams
	err := json.Unmarshal(request.Params, &params)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	// Order by URI for pagination
	var orderedResources []*resource
	s.resources.Range(func(k string, r *resource) bool {
		orderedResources = append(orderedResources, r)
		return true
	})
	sort.Slice(orderedResources, func(i, j int) bool {
		return orderedResources[i].Uri < orderedResources[j].Uri
	})

	startPosition := 0
	if params.Cursor != nil {
		// Base64 decode the cursor
		c, err := base64.StdEncoding.DecodeString(*params.Cursor)
		if err != nil {
			return nil, fmt.Errorf("failed to decode cursor: %w", err)
		}
		cString := string(c)
		// Iterate through the resources until we find an entry > the cursor
		for i := 0; i < len(orderedResources); i++ {
			if orderedResources[i].Uri > cString {
				startPosition = i
				break
			}
		}
	}
	endPosition := len(orderedResources)
	if s.paginationLimit != nil {
		// Make sure we don't go out of bounds
		if len(orderedResources) > startPosition+*s.paginationLimit {
			endPosition = startPosition + *s.paginationLimit
		}
	}

	resourcesToReturn := make([]*resourceSchema, 0)
	for i := startPosition; i < endPosition; i++ {
		r := orderedResources[i]
		resourcesToReturn = append(resourcesToReturn, &resourceSchema{
			Annotations: nil,
			Description: &r.Description,
			MimeType:    &r.mimeType,
			Name:        r.Name,
			Uri:         r.Uri,
		})
	}

	return listResourcesResult{
		Resources: resourcesToReturn,
		NextCursor: func() *string {
			if s.paginationLimit != nil && len(resourcesToReturn) >= *s.paginationLimit {
				toString := base64.StdEncoding.EncodeToString([]byte(resourcesToReturn[len(resourcesToReturn)-1].Uri))
				return &toString
			}
			return nil
		}(),
	}, nil
}

func (s *Server) handlePromptCalls(req *transport.BaseJSONRPCRequest, extra protocol.RequestHandlerExtra) (transport.JsonRpcBody, error) {
	params := baseGetPromptRequestParamsArguments{}
	// Instantiate a struct of the type of the arguments
	err := json.Unmarshal(req.Params, &params)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	var promptToUse *prompt
	s.prompts.Range(func(k string, p *prompt) bool {
		if k != params.Name {
			return true
		}
		promptToUse = p
		return false
	})

	if promptToUse == nil {
		return nil, fmt.Errorf("unknown prompt: %s", req.Method)
	}
	return promptToUse.Handler(params), nil
}

func (s *Server) handleResourceCalls(req *transport.BaseJSONRPCRequest, extra protocol.RequestHandlerExtra) (transport.JsonRpcBody, error) {
	params := readResourceRequestParams{}
	// Instantiate a struct of the type of the arguments
	err := json.Unmarshal(req.Params, &params)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	var resourceToUse *resource
	s.resources.Range(func(k string, r *resource) bool {
		if k != params.Uri {
			return true
		}
		resourceToUse = r
		return false
	})

	if resourceToUse == nil {
		return nil, fmt.Errorf("unknown prompt: %s", req.Method)
	}
	return resourceToUse.Handler(), nil
}

func (s *Server) handlePing(request *transport.BaseJSONRPCRequest, extra protocol.RequestHandlerExtra) (transport.JsonRpcBody, error) {
	return map[string]interface{}{}, nil
}

func validateToolHandler(handler any) error {
	handlerValue := reflect.ValueOf(handler)
	handlerType := handlerValue.Type()

	if handlerType.NumIn() != 1 {
		return fmt.Errorf("handler must take exactly one argument, got %d", handlerType.NumIn())
	}

	if handlerType.NumOut() != 2 {
		return fmt.Errorf("handler must return exactly two values, got %d", handlerType.NumOut())
	}

	// Check that the output type is *tools.ToolResponse
	if handlerType.Out(0) != reflect.PointerTo(reflect.TypeOf(ToolResponse{})) {
		return fmt.Errorf("handler must return *tools.ToolResponse, got %s", handlerType.Out(0).Name())
	}

	// Check that the output type is error
	if handlerType.Out(1) != reflect.TypeOf((*error)(nil)).Elem() {
		return fmt.Errorf("handler must return error, got %s", handlerType.Out(1).Name())
	}

	return nil
}

var (
	jsonSchemaReflector = jsonschema.Reflector{
		BaseSchemaID:               "",
		Anonymous:                  true,
		AssignAnchor:               false,
		AllowAdditionalProperties:  true,
		RequiredFromJSONSchemaTags: true,
		DoNotReference:             true,
		ExpandedStruct:             true,
		FieldNameTag:               "",
		IgnoredTypes:               nil,
		Lookup:                     nil,
		Mapper:                     nil,
		Namer:                      nil,
		KeyNamer:                   nil,
		AdditionalFields:           nil,
		CommentMap:                 nil,
	}
)
