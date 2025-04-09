package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/metoro-io/mcp-golang/transport"
	mcphttp "github.com/metoro-io/mcp-golang/transport/http"
)

// GinTransport implements a stateless HTTP transport for MCP using Gin
type GinTransport struct {
	base *mcphttp.BaseTransport
}

// NewGinTransport creates a new Gin transport
func NewGinTransport() *GinTransport {
	return &GinTransport{
		base: mcphttp.NewBaseTransport(),
	}
}

// Start implements Transport.Start - no-op for Gin transport as it's handled by Gin
func (t *GinTransport) Start(ctx context.Context) error {
	return nil
}

// Send implements Transport.Send
func (t *GinTransport) Send(ctx context.Context, message *transport.BaseJsonRpcMessage) error {
	key := message.JsonRpcResponse.Id
	responseChannel := t.base.ResponseMap[int64(key)]
	if responseChannel == nil {
		return fmt.Errorf("no response channel found for key: %d", key)
	}
	responseChannel <- message
	return nil
}

// Close implements Transport.Close
func (t *GinTransport) Close() error {
	if t.base.CloseHandler != nil {
		t.base.CloseHandler()
	}
	return nil
}

// SetCloseHandler implements Transport.SetCloseHandler
func (t *GinTransport) SetCloseHandler(handler func()) {
	t.base.CloseHandler = handler
}

// SetErrorHandler implements Transport.SetErrorHandler
func (t *GinTransport) SetErrorHandler(handler func(error)) {
	t.base.ErrorHandler = handler
}

// SetMessageHandler implements Transport.SetMessageHandler
func (t *GinTransport) SetMessageHandler(handler func(ctx context.Context, message *transport.BaseJsonRpcMessage)) {
	t.base.MessageHandler = handler
}

// Handler returns a Gin handler function that can be used with Gin's router
func (t *GinTransport) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		ctx = context.WithValue(ctx, "ginContext", c)
		if c.Request.Method != http.MethodPost {
			c.String(http.StatusMethodNotAllowed, "Only POST method is supported")
			return
		}

		body, err := t.base.ReadBody(c.Request.Body)
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}

		response, err := t.base.HandleMessage(ctx, body)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		jsonData, err := json.Marshal(response)
		if err != nil {
			if t.base.ErrorHandler != nil {
				t.base.ErrorHandler(fmt.Errorf("failed to marshal response: %w", err))
			}
			c.String(http.StatusInternalServerError, "Failed to marshal response")
			return
		}

		c.Data(http.StatusOK, "application/json", jsonData)
	}
}
