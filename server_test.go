package mcp_golang

import (
	"github.com/metoro-io/mcp-golang/internal/testingutils"
	"testing"
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
