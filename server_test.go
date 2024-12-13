package mcp_golang

import (
	"testing"

	"github.com/metoro-io/mcp-golang/internal/protocol"
	"github.com/metoro-io/mcp-golang/internal/testingutils"
	"github.com/metoro-io/mcp-golang/internal/tools"
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
	resp, err := server.handleListTools(&transport.BaseJSONRPCRequest{
		Params: []byte(`{}`),
	}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	toolsResp, ok := resp.(tools.ToolsResponse)
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
	resp, err = server.handleListTools(&transport.BaseJSONRPCRequest{
		Params: []byte(`{"cursor":"` + *toolsResp.NextCursor + `"}`),
	}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	toolsResp, ok = resp.(tools.ToolsResponse)
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
	resp, err = server.handleListTools(&transport.BaseJSONRPCRequest{
		Params: []byte(`{"cursor":"` + *toolsResp.NextCursor + `"}`),
	}, protocol.RequestHandlerExtra{})
	if err != nil {
		t.Fatal(err)
	}

	toolsResp, ok = resp.(tools.ToolsResponse)
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
	_, err = server.handleListTools(&transport.BaseJSONRPCRequest{
		Params: []byte(`{"cursor":"invalid-cursor"}`),
	}, protocol.RequestHandlerExtra{})
	if err == nil {
		t.Error("Expected error for invalid cursor")
	}
}
