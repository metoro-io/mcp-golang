package mcp_golang

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/metoro-io/mcp-golang/internal/testingutils"
	"github.com/metoro-io/mcp-golang/transport"
	"github.com/stretchr/testify/assert"
)

func TestClient_ReadResource(t *testing.T) {
	// Create a mock transport
	mockTransport := testingutils.NewMockTransport()

	// Create the client
	client := NewClient(mockTransport)

	// Connect the transport (normally done in Initialize)
	err := client.protocol.Connect(mockTransport)
	assert.NoError(t, err)
	client.initialized = true // Skip full initialization

	// Sample URI for the test
	uri := "config://app"

	// Set up a message handler to simulate server response
	mockTransport.SetMessageHandler(func(ctx context.Context, message *transport.BaseJsonRpcMessage) {
		// Extract the request details
		req := message.JsonRpcRequest

		// Verify correct method and URI
		if req.Method != "resources/read" {
			t.Errorf("Expected method resources/read, got %s", req.Method)
			return
		}

		// Create sample response
		sampleResource := NewTextEmbeddedResource(
			uri,
			`{"app_name":"MyExampleServer","version":"1.0.0"}`,
			"application/json",
		)

		responseObj := &ResourceResponse{
			Contents: []*EmbeddedResource{sampleResource},
		}

		// Marshal the response
		responseBytes, err := json.Marshal(responseObj)
		assert.NoError(t, err)

		// Send the response back
		mockTransport.SimulateMessage(transport.NewBaseMessageResponse(&transport.BaseJSONRPCResponse{
			Jsonrpc: "2.0",
			Id:      req.Id,
			Result:  responseBytes,
		}))
	})

	// Call the method
	response, err := client.ReadResource(context.Background(), uri)

	// Verify the results
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 1, len(response.Contents))
	assert.NotNil(t, response.Contents[0].TextResourceContents)
	assert.Equal(t, uri, response.Contents[0].TextResourceContents.Uri)
	assert.Equal(t, `{"app_name":"MyExampleServer","version":"1.0.0"}`, response.Contents[0].TextResourceContents.Text)
}

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}

// TestResourceResponseUnmarshaling tests the fix for ReadResource by directly testing the unmarshaling
func TestResourceResponseUnmarshaling(t *testing.T) {
	// This sample represents the response format from the server
	sampleResponse := `{
		"contents": [
			{
				"uri": "config://app",
				"text": "{\"app_name\":\"MyExampleServer\",\"version\":\"1.0.0\"}",
				"mimeType": "application/json"
			}
		]
	}`

	// Parse directly into ResourceResponse
	var resourceResponse ResourceResponse
	err := json.Unmarshal([]byte(sampleResponse), &resourceResponse)
	assert.NoError(t, err)

	// Verify the results
	assert.NotNil(t, resourceResponse)
	assert.Equal(t, 1, len(resourceResponse.Contents))
	assert.NotNil(t, resourceResponse.Contents[0].TextResourceContents)
	assert.Equal(t, "config://app", resourceResponse.Contents[0].TextResourceContents.Uri)
	assert.Equal(t, "{\"app_name\":\"MyExampleServer\",\"version\":\"1.0.0\"}", resourceResponse.Contents[0].TextResourceContents.Text)
}
