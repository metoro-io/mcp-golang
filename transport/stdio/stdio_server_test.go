package stdio

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rvoh-emccaleb/mcp-golang/transport"

	"github.com/stretchr/testify/assert"
)

func TestStdioServerTransport(t *testing.T) {
	t.Run("basic message handling", func(t *testing.T) {
		in := &bytes.Buffer{}
		out := &bytes.Buffer{}
		tr := NewStdioServerTransportWithIO(in, out)

		var receivedMsg transport.JSONRPCMessage
		var wg sync.WaitGroup
		wg.Add(1)

		ctx := context.Background()

		tr.SetMessageHandler(func(ctx context.Context, msg *transport.BaseJsonRpcMessage) {
			receivedMsg = msg
			wg.Done()
		})

		err := tr.Start(ctx)
		assert.NoError(t, err)

		// Write a test message to the input buffer
		testMsg := `{"jsonrpc": "2.0", "method": "test", "params": {}, "id": 1}` + "\n"
		_, err = in.Write([]byte(testMsg))
		assert.NoError(t, err)

		// Wait for message processing with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for message")
		}

		// Verify received message
		req, ok := receivedMsg.(*transport.BaseJsonRpcMessage)
		assert.True(t, ok)
		assert.True(t, req.Type == transport.BaseMessageTypeJSONRPCRequestType)
		assert.Equal(t, "test", req.JsonRpcRequest.Method)
		assert.Equal(t, transport.RequestId(1), req.JsonRpcRequest.Id)

		err = tr.Close()
		assert.NoError(t, err)
	})

	t.Run("double start error", func(t *testing.T) {
		transport := NewStdioServerTransport()
		ctx := context.Background()
		err := transport.Start(ctx)
		assert.NoError(t, err)

		err = transport.Start(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already started")

		err = transport.Close()
		assert.NoError(t, err)
	})

	t.Run("send message", func(t *testing.T) {
		in := &bytes.Buffer{}
		out := &bytes.Buffer{}
		tr := NewStdioServerTransportWithIO(in, out)

		result := []byte(`{"status": "ok"}`)

		msg := &transport.BaseJSONRPCResponse{
			Jsonrpc: "2.0",
			Result:  result,
			Id:      1,
		}

		err := tr.Send(context.Background(), transport.NewBaseMessageResponse(msg))
		assert.NoError(t, err)

		// Verify output contains the message and newline
		assert.Contains(t, out.String(), `{"id":1,"jsonrpc":"2.0","result":{"status":"ok"}}`)
		assert.Contains(t, out.String(), "\n")
	})

	t.Run("error handling", func(t *testing.T) {
		in := &bytes.Buffer{}
		out := &bytes.Buffer{}
		transport := NewStdioServerTransportWithIO(in, out)

		var receivedErr error
		var wg sync.WaitGroup
		wg.Add(1)

		transport.SetErrorHandler(func(err error) {
			receivedErr = err
			wg.Done()
		})

		ctx := context.Background()
		err := transport.Start(ctx)
		assert.NoError(t, err)

		// Write invalid JSON to trigger error
		_, err = in.Write([]byte(`{"invalid json`))
		assert.NoError(t, err)

		// Write newline to complete the message
		_, err = in.Write([]byte("\n"))
		assert.NoError(t, err)

		// Wait for error handling with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for error")
		}

		assert.NotNil(t, receivedErr)
		assert.Contains(t, receivedErr.Error(), "failed to unmarshal JSON-RPC message, unrecognized type")

		err = transport.Close()
		assert.NoError(t, err)
	})

	t.Run("context cancellation", func(t *testing.T) {
		in := &bytes.Buffer{}
		out := &bytes.Buffer{}
		transport := NewStdioServerTransportWithIO(in, out)

		ctx, cancel := context.WithCancel(context.Background())
		err := transport.Start(ctx)
		assert.NoError(t, err)

		var closed bool
		transport.SetCloseHandler(func() {
			closed = true
		})

		// Cancel context and wait for close
		cancel()
		time.Sleep(100 * time.Millisecond)

		assert.True(t, closed, "transport should be closed after context cancellation")
	})
}
