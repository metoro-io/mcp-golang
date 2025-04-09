package mcp_golang

import (
	"context"
	"testing"

	"github.com/metoro-io/mcp-golang/internal/protocol"
	"github.com/metoro-io/mcp-golang/internal/testingutils"
	"github.com/metoro-io/mcp-golang/transport"
)

func TestServerListChangedNotifications(t *testing.T) {
	mockTransport := testingutils.NewMockTransport()
	server := NewServer(mockTransport)
	err := server.Serve()
	if err != nil {
		t.Fatal(err)
	}

	// Test tool registration notification
	type TestToolArgs struct {
		Message string `json:"message" jsonschema:"required,description=A test message"`
	}
	err = server.RegisterTool("test-tool", "Test tool", func(args TestToolArgs) (*ToolResponse, error) {
		return NewToolResponse(), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	messages := mockTransport.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message after tool registration, got %d", len(messages))
	}
	if messages[0].JsonRpcNotification.Method != "notifications/tools/list_changed" {
		t.Errorf("Expected tools list changed notification, got %s", messages[0].JsonRpcNotification.Method)
	}

	// Test tool deregistration notification
	mockTransport = testingutils.NewMockTransport()
	server = NewServer(mockTransport)
	err = server.Serve()
	if err != nil {
		t.Fatal(err)
	}
	err = server.RegisterTool("test-tool", "Test tool", func(args TestToolArgs) (*ToolResponse, error) {
		return NewToolResponse(), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = server.DeregisterTool("test-tool")
	if err != nil {
		t.Fatal(err)
	}
	messages = mockTransport.GetMessages()
	if len(messages) != 2 {
		t.Fatalf("Expected 2 messages after tool registration and deregistration, got %d", len(messages))
	}
	if messages[1].JsonRpcNotification.Method != "notifications/tools/list_changed" {
		t.Errorf("Expected tools list changed notification, got %s", messages[1].JsonRpcNotification.Method)
	}

	// Test prompt registration notification
	type TestPromptArgs struct {
		Query string `json:"query" jsonschema:"required,description=A test query"`
	}
	mockTransport = testingutils.NewMockTransport()
	server = NewServer(mockTransport)
	err = server.Serve()
	if err != nil {
		t.Fatal(err)
	}
	err = server.RegisterPrompt("test-prompt", "Test prompt", func(args TestPromptArgs) (*PromptResponse, error) {
		return NewPromptResponse("test", NewPromptMessage(NewTextContent("test"), RoleUser)), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	messages = mockTransport.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message after prompt registration, got %d", len(messages))
	}
	if messages[0].JsonRpcNotification.Method != "notifications/prompts/list_changed" {
		t.Errorf("Expected prompts list changed notification, got %s", messages[0].JsonRpcNotification.Method)
	}

	// Test prompt deregistration notification
	mockTransport = testingutils.NewMockTransport()
	server = NewServer(mockTransport)
	err = server.Serve()
	if err != nil {
		t.Fatal(err)
	}
	err = server.RegisterPrompt("test-prompt", "Test prompt", func(args TestPromptArgs) (*PromptResponse, error) {
		return NewPromptResponse("test", NewPromptMessage(NewTextContent("test"), RoleUser)), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = server.DeregisterPrompt("test-prompt")
	if err != nil {
		t.Fatal(err)
	}
	messages = mockTransport.GetMessages()
	if len(messages) != 2 {
		t.Fatalf("Expected 2 messages after prompt registration and deregistration, got %d", len(messages))
	}
	if messages[1].JsonRpcNotification.Method != "notifications/prompts/list_changed" {
		t.Errorf("Expected prompts list changed notification, got %s", messages[1].JsonRpcNotification.Method)
	}

	// Test resource registration notification
	mockTransport = testingutils.NewMockTransport()
	server = NewServer(mockTransport)
	err = server.Serve()
	if err != nil {
		t.Fatal(err)
	}
	err = server.RegisterResource("test://resource", "test-resource", "Test resource", "text/plain", func() (*ResourceResponse, error) {
		return NewResourceResponse(NewTextEmbeddedResource("test://resource", "test content", "text/plain")), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	messages = mockTransport.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message after resource registration, got %d", len(messages))
	}
	if messages[0].JsonRpcNotification.Method != "notifications/resources/list_changed" {
		t.Errorf("Expected resources list changed notification, got %s", messages[0].JsonRpcNotification.Method)
	}

	// Test resource deregistration notification
	mockTransport = testingutils.NewMockTransport()
	server = NewServer(mockTransport)
	err = server.Serve()
	if err != nil {
		t.Fatal(err)
	}
	err = server.RegisterResource("test://resource", "test-resource", "Test resource", "text/plain", func() (*ResourceResponse, error) {
		return NewResourceResponse(NewTextEmbeddedResource("test://resource", "test content", "text/plain")), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = server.DeregisterResource("test://resource")
	if err != nil {
		t.Fatal(err)
	}
	messages = mockTransport.GetMessages()
	if len(messages) != 2 {
		t.Fatalf("Expected 2 messages after resource registration and deregistration, got %d", len(messages))
	}
	if messages[1].JsonRpcNotification.Method != "notifications/resources/list_changed" {
		t.Errorf("Expected resources list changed notification, got %s", messages[1].JsonRpcNotification.Method)
	}
}

func TestHandleListToolsPagination(t *testing.T) {
	mockTransport := testingutils.NewMockTransport()
	server := NewServer(mockTransport)
	err := server.Serve()
	if err != nil {
		t.Fatal(err)
	}

	// Register tools in a non alphabetical order
	toolNames := []string{"b-tool", "a-tool", "c-tool", "e-tool", "d-tool"}
	type testToolArgs struct {
		Message string `json:"message" jsonschema:"required,description=A test message"`
	}
	for _, name := range toolNames {
		err = server.RegisterTool(name, "Test tool "+name, func(args testToolArgs) (*ToolResponse, error) {
			return NewToolResponse(), nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Set pagination limit to 2 items per page
	limit := 2
	server.paginationLimit = &limit

	// Test first page (no cursor)
	resp, err := server.handleListTools(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{}`),
	}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	toolsResp, ok := resp.(ToolsResponse)
	if !ok {
		t.Fatal("Expected tools.ToolsResponse")
	}

	// Verify first page
	if len(toolsResp.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(toolsResp.Tools))
	}
	if toolsResp.Tools[0].Name != "a-tool" || toolsResp.Tools[1].Name != "b-tool" {
		t.Errorf("Unexpected tools in first page: %v", toolsResp.Tools)
	}
	if toolsResp.NextCursor == nil {
		t.Fatal("Expected next cursor for first page")
	}

	// Test second page
	resp, err = server.handleListTools(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{"cursor":"` + *toolsResp.NextCursor + `"}`),
	}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	toolsResp, ok = resp.(ToolsResponse)
	if !ok {
		t.Fatal("Expected tools.ToolsResponse")
	}

	// Verify second page
	if len(toolsResp.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(toolsResp.Tools))
	}
	if toolsResp.Tools[0].Name != "c-tool" || toolsResp.Tools[1].Name != "d-tool" {
		t.Errorf("Unexpected tools in second page: %v", toolsResp.Tools)
	}
	if toolsResp.NextCursor == nil {
		t.Fatal("Expected next cursor for second page")
	}

	// Test last page
	resp, err = server.handleListTools(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{"cursor":"` + *toolsResp.NextCursor + `"}`),
	}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	toolsResp, ok = resp.(ToolsResponse)
	if !ok {
		t.Fatal("Expected tools.ToolsResponse")
	}

	// Verify last page
	if len(toolsResp.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(toolsResp.Tools))
	}
	if toolsResp.Tools[0].Name != "e-tool" {
		t.Errorf("Unexpected tool in last page: %v", toolsResp.Tools)
	}
	if toolsResp.NextCursor != nil {
		t.Error("Expected no next cursor for last page")
	}

	// Test invalid cursor
	_, err = server.handleListTools(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{"cursor":"invalid-cursor"}`),
	}, protocol.RequestHandlerExtra{})
	if err == nil {
		t.Error("Expected error for invalid cursor")
	}

	// Test without pagination (should return all tools)
	server.paginationLimit = nil
	resp, err = server.handleListTools(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{}`),
	}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	toolsResp, ok = resp.(ToolsResponse)
	if !ok {
		t.Fatal("Expected ToolsResponse")
	}

	if len(toolsResp.Tools) != 5 {
		t.Errorf("Expected 5 tools, got %d", len(toolsResp.Tools))
	}
	if toolsResp.NextCursor != nil {
		t.Error("Expected no next cursor when pagination is disabled")
	}
}

func TestHandleListPromptsPagination(t *testing.T) {
	mockTransport := testingutils.NewMockTransport()
	server := NewServer(mockTransport)
	err := server.Serve()
	if err != nil {
		t.Fatal(err)
	}

	// Register prompts in a non alphabetical order
	promptNames := []string{"b-prompt", "a-prompt", "c-prompt", "e-prompt", "d-prompt"}
	type testPromptArgs struct {
		Message string `json:"message" jsonschema:"required,description=A test message"`
	}
	for _, name := range promptNames {
		err = server.RegisterPrompt(name, "Test prompt "+name, func(args testPromptArgs) (*PromptResponse, error) {
			return NewPromptResponse("test", NewPromptMessage(NewTextContent("test"), RoleUser)), nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Set pagination limit to 2 items per page
	limit := 2
	server.paginationLimit = &limit

	// Test first page (no cursor)
	resp, err := server.handleListPrompts(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{}`),
	}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	promptsResp, ok := resp.(ListPromptsResponse)
	if !ok {
		t.Fatal("Expected listPromptsResult")
	}

	// Verify first page
	if len(promptsResp.Prompts) != 2 {
		t.Errorf("Expected 2 prompts, got %d", len(promptsResp.Prompts))
	}
	if promptsResp.Prompts[0].Name != "a-prompt" || promptsResp.Prompts[1].Name != "b-prompt" {
		t.Errorf("Unexpected prompts in first page: %v", promptsResp.Prompts)
	}
	if promptsResp.NextCursor == nil {
		t.Fatal("Expected next cursor for first page")
	}

	// Test second page
	resp, err = server.handleListPrompts(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{"cursor":"` + *promptsResp.NextCursor + `"}`),
	}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	promptsResp, ok = resp.(ListPromptsResponse)
	if !ok {
		t.Fatal("Expected listPromptsResult")
	}

	// Verify second page
	if len(promptsResp.Prompts) != 2 {
		t.Errorf("Expected 2 prompts, got %d", len(promptsResp.Prompts))
	}
	if promptsResp.Prompts[0].Name != "c-prompt" || promptsResp.Prompts[1].Name != "d-prompt" {
		t.Errorf("Unexpected prompts in second page: %v", promptsResp.Prompts)
	}
	if promptsResp.NextCursor == nil {
		t.Fatal("Expected next cursor for second page")
	}

	// Test last page
	resp, err = server.handleListPrompts(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{"cursor":"` + *promptsResp.NextCursor + `"}`),
	}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	promptsResp, ok = resp.(ListPromptsResponse)
	if !ok {
		t.Fatal("Expected listPromptsResult")
	}

	// Verify last page
	if len(promptsResp.Prompts) != 1 {
		t.Errorf("Expected 1 prompt, got %d", len(promptsResp.Prompts))
	}
	if promptsResp.Prompts[0].Name != "e-prompt" {
		t.Errorf("Unexpected prompt in last page: %v", promptsResp.Prompts)
	}
	if promptsResp.NextCursor != nil {
		t.Error("Expected no next cursor for last page")
	}

	// Test invalid cursor
	_, err = server.handleListPrompts(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{"cursor":"invalid-cursor"}`),
	}, protocol.RequestHandlerExtra{})
	if err == nil {
		t.Error("Expected error for invalid cursor")
	}

	// Test without pagination (should return all prompts)
	server.paginationLimit = nil
	resp, err = server.handleListPrompts(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{}`),
	}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	promptsResp, ok = resp.(ListPromptsResponse)
	if !ok {
		t.Fatal("Expected listPromptsResult")
	}

	if len(promptsResp.Prompts) != 5 {
		t.Errorf("Expected 5 prompts, got %d", len(promptsResp.Prompts))
	}
	if promptsResp.NextCursor != nil {
		t.Error("Expected no next cursor when pagination is disabled")
	}
}

func TestHandleListResourcesNoParams(t *testing.T) {
	mockTransport := testingutils.NewMockTransport()
	server := NewServer(mockTransport)
	err := server.Serve()
	if err != nil {
		t.Fatal(err)
	}

	// Register resources
	resourceURIs := []string{"b://resource", "a://resource"}
	for _, uri := range resourceURIs {
		err = server.RegisterResource(uri, "resource-"+uri, "Test resource "+uri, "text/plain", func() (*ResourceResponse, error) {
			return NewResourceResponse(NewTextEmbeddedResource(uri, "test content", "text/plain")), nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Test with no Params defined
	resp, err := server.handleListResources(context.Background(), &transport.BaseJSONRPCRequest{}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	resourcesResp, ok := resp.(ListResourcesResponse)
	if !ok {
		t.Fatal("Expected ListResourcesResponse")
	}

	// Verify empty resources list
	if len(resourcesResp.Resources) != len(resourceURIs) {
		t.Errorf("Expected %d resources, got %d", len(resourceURIs), len(resourcesResp.Resources))
	}
}

func TestHandleListResourcesPagination(t *testing.T) {
	mockTransport := testingutils.NewMockTransport()
	server := NewServer(mockTransport)
	err := server.Serve()
	if err != nil {
		t.Fatal(err)
	}

	// Register resources in a non alphabetical order
	resourceURIs := []string{"b://resource", "a://resource", "c://resource", "e://resource", "d://resource"}
	for _, uri := range resourceURIs {
		err = server.RegisterResource(uri, "resource-"+uri, "Test resource "+uri, "text/plain", func() (*ResourceResponse, error) {
			return NewResourceResponse(NewTextEmbeddedResource(uri, "test content", "text/plain")), nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Set pagination limit to 2 items per page
	limit := 2
	server.paginationLimit = &limit

	// Test first page (no cursor)
	resp, err := server.handleListResources(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{}`),
	}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	resourcesResp, ok := resp.(ListResourcesResponse)
	if !ok {
		t.Fatal("Expected listResourcesResult")
	}

	// Verify first page
	if len(resourcesResp.Resources) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(resourcesResp.Resources))
	}
	if resourcesResp.Resources[0].Uri != "a://resource" || resourcesResp.Resources[1].Uri != "b://resource" {
		t.Errorf("Unexpected resources in first page: %v", resourcesResp.Resources)
	}
	if resourcesResp.NextCursor == nil {
		t.Fatal("Expected next cursor for first page")
	}

	// Test second page
	resp, err = server.handleListResources(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{"cursor":"` + *resourcesResp.NextCursor + `"}`),
	}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	resourcesResp, ok = resp.(ListResourcesResponse)
	if !ok {
		t.Fatal("Expected listResourcesResult")
	}

	// Verify second page
	if len(resourcesResp.Resources) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(resourcesResp.Resources))
	}
	if resourcesResp.Resources[0].Uri != "c://resource" || resourcesResp.Resources[1].Uri != "d://resource" {
		t.Errorf("Unexpected resources in second page: %v", resourcesResp.Resources)
	}
	if resourcesResp.NextCursor == nil {
		t.Fatal("Expected next cursor for second page")
	}

	// Test last page
	resp, err = server.handleListResources(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{"cursor":"` + *resourcesResp.NextCursor + `"}`),
	}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	resourcesResp, ok = resp.(ListResourcesResponse)
	if !ok {
		t.Fatal("Expected listResourcesResult")
	}

	// Verify last page
	if len(resourcesResp.Resources) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(resourcesResp.Resources))
	}
	if resourcesResp.Resources[0].Uri != "e://resource" {
		t.Errorf("Unexpected resource in last page: %v", resourcesResp.Resources)
	}
	if resourcesResp.NextCursor != nil {
		t.Error("Expected no next cursor for last page")
	}

	// Test invalid cursor
	_, err = server.handleListResources(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{"cursor":"invalid-cursor"}`),
	}, protocol.RequestHandlerExtra{})
	if err == nil {
		t.Error("Expected error for invalid cursor")
	}

	// Test without pagination (should return all resources)
	server.paginationLimit = nil
	resp, err = server.handleListResources(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{}`),
	}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	resourcesResp, ok = resp.(ListResourcesResponse)
	if !ok {
		t.Fatal("Expected listResourcesResult")
	}

	if len(resourcesResp.Resources) != 5 {
		t.Errorf("Expected 5 resources, got %d", len(resourcesResp.Resources))
	}
	if resourcesResp.NextCursor != nil {
		t.Error("Expected no next cursor when pagination is disabled")
	}
}

func TestHandleListResourceTemplatesPagination(t *testing.T) {
	mockTransport := testingutils.NewMockTransport()
	server := NewServer(mockTransport)
	err := server.Serve()
	if err != nil {
		t.Fatal(err)
	}

	// Register templates in a non alphabetical order
	templateURIs := []string{
		"b://{param}/resource",
		"a://{param}/resource",
		"c://{param}/resource",
		"e://{param}/resource",
		"d://{param}/resource",
	}
	for _, uri := range templateURIs {
		err = server.RegisterResourceTemplate(
			uri,
			"template-"+uri,
			"Test template "+uri,
			"text/plain",
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Set pagination limit to 2 items per page
	limit := 2
	server.paginationLimit = &limit

	// Test first page (no cursor)
	resp, err := server.handleListResourceTemplates(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{}`),
	}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	templatesResp, ok := resp.(ListResourceTemplatesResponse)
	if !ok {
		t.Fatal("Expected ListResourceTemplatesResponse")
	}

	// Verify first page
	if len(templatesResp.Templates) != 2 {
		t.Errorf("Expected 2 templates, got %d", len(templatesResp.Templates))
	}
	if templatesResp.Templates[0].UriTemplate != "a://{param}/resource" || templatesResp.Templates[1].UriTemplate != "b://{param}/resource" {
		t.Errorf("Unexpected templates in first page: %v", templatesResp.Templates)
	}
	if templatesResp.NextCursor == nil {
		t.Fatal("Expected next cursor for first page")
	}

	// Test second page
	resp, err = server.handleListResourceTemplates(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{"cursor":"` + *templatesResp.NextCursor + `"}`),
	}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	templatesResp, ok = resp.(ListResourceTemplatesResponse)
	if !ok {
		t.Fatal("Expected ListResourceTemplatesResponse")
	}

	// Verify second page
	if len(templatesResp.Templates) != 2 {
		t.Errorf("Expected 2 templates, got %d", len(templatesResp.Templates))
	}
	if templatesResp.Templates[0].UriTemplate != "c://{param}/resource" || templatesResp.Templates[1].UriTemplate != "d://{param}/resource" {
		t.Errorf("Unexpected templates in second page: %v", templatesResp.Templates)
	}
	if templatesResp.NextCursor == nil {
		t.Fatal("Expected next cursor for second page")
	}

	// Test last page
	resp, err = server.handleListResourceTemplates(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{"cursor":"` + *templatesResp.NextCursor + `"}`),
	}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	templatesResp, ok = resp.(ListResourceTemplatesResponse)
	if !ok {
		t.Fatal("Expected ListResourceTemplatesResponse")
	}

	// Verify last page
	if len(templatesResp.Templates) != 1 {
		t.Errorf("Expected 1 template, got %d", len(templatesResp.Templates))
	}
	if templatesResp.Templates[0].UriTemplate != "e://{param}/resource" {
		t.Errorf("Unexpected template in last page: %v", templatesResp.Templates)
	}
	if templatesResp.NextCursor != nil {
		t.Error("Expected no next cursor for last page")
	}

	// Test invalid cursor
	_, err = server.handleListResourceTemplates(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{"cursor":"invalid-cursor"}`),
	}, protocol.RequestHandlerExtra{})
	if err == nil {
		t.Error("Expected error for invalid cursor")
	}

	// Test without pagination (should return all templates)
	server.paginationLimit = nil
	resp, err = server.handleListResourceTemplates(context.Background(), &transport.BaseJSONRPCRequest{
		Params: []byte(`{}`),
	}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	templatesResp, ok = resp.(ListResourceTemplatesResponse)
	if !ok {
		t.Fatal("Expected ListResourceTemplatesResponse")
	}

	if len(templatesResp.Templates) != 5 {
		t.Errorf("Expected 5 templates, got %d", len(templatesResp.Templates))
	}
	if templatesResp.NextCursor != nil {
		t.Error("Expected no next cursor when pagination is disabled")
	}
}
