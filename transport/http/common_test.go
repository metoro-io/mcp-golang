package http

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/metoro-io/mcp-golang/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseTransportSendNoChannelFound(t *testing.T) {
	tr := newBaseTransport()

	err := tr.Send(context.Background(), newResponseMessage(1))
	require.Error(t, err)
	require.Contains(t, err.Error(), "no response channel found for key: 1")
}

func TestBaseTransportHandleMessageConcurrentRequests(t *testing.T) {
	tr := newBaseTransport()

	const requestCount = 200
	errs := make(chan error, requestCount*2)

	tr.SetMessageHandler(func(ctx context.Context, message *transport.BaseJsonRpcMessage) {
		if message.Type != transport.BaseMessageTypeJSONRPCRequestType || message.JsonRpcRequest == nil {
			errs <- fmt.Errorf("unexpected message type: %s", message.Type)
			return
		}

		requestID := message.JsonRpcRequest.Id
		go func() {
			if err := tr.Send(ctx, newResponseMessage(requestID)); err != nil {
				errs <- fmt.Errorf("failed sending response for id %d: %w", requestID, err)
			}
		}()
	})

	var wg sync.WaitGroup
	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			originalID := transport.RequestId(10000 + i)
			body, err := marshalRequestBody(originalID)
			if err != nil {
				errs <- fmt.Errorf("failed marshalling request %d: %w", originalID, err)
				return
			}

			response, err := tr.handleMessage(context.Background(), body)
			if err != nil {
				errs <- fmt.Errorf("handleMessage failed for %d: %w", originalID, err)
				return
			}

			if response == nil || response.JsonRpcResponse == nil {
				errs <- fmt.Errorf("missing response for request %d", originalID)
				return
			}

			if response.JsonRpcResponse.Id != originalID {
				errs <- fmt.Errorf("response id mismatch: got %d want %d", response.JsonRpcResponse.Id, originalID)
			}
		}(i)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	assert.Empty(t, tr.getResponseMapKeys())
}

func TestHTTPTransportSendConcurrent(t *testing.T) {
	tr := NewHTTPTransport("/mcp")
	runConcurrentSendTest(t, tr.baseTransport, tr.Send)
}

func TestGinTransportSendConcurrent(t *testing.T) {
	tr := NewGinTransport()
	runConcurrentSendTest(t, tr.baseTransport, tr.Send)
}

func runConcurrentSendTest(
	t *testing.T,
	base *baseTransport,
	sendFn func(context.Context, *transport.BaseJsonRpcMessage) error,
) {
	t.Helper()

	const requestCount = 200
	channels := make(map[int64]chan *transport.BaseJsonRpcMessage, requestCount)

	base.mu.Lock()
	for i := 0; i < requestCount; i++ {
		key := int64(i)
		responseChannel := make(chan *transport.BaseJsonRpcMessage, 1)
		base.responseMap[key] = responseChannel
		channels[key] = responseChannel
	}
	base.mu.Unlock()

	errs := make(chan error, requestCount)
	var wg sync.WaitGroup

	for i := 0; i < requestCount; i++ {
		key := int64(i)
		wg.Add(1)
		go func(key int64) {
			defer wg.Done()
			if err := sendFn(context.Background(), newResponseMessage(transport.RequestId(key))); err != nil {
				errs <- fmt.Errorf("send failed for key %d: %w", key, err)
			}
		}(key)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	for key, responseChannel := range channels {
		select {
		case msg := <-responseChannel:
			require.NotNil(t, msg)
			require.NotNil(t, msg.JsonRpcResponse)
			assert.Equal(t, transport.RequestId(key), msg.JsonRpcResponse.Id)
		default:
			t.Fatalf("expected response for key %d", key)
		}
		base.deleteResponseChannel(key)
	}
}

func newResponseMessage(id transport.RequestId) *transport.BaseJsonRpcMessage {
	return transport.NewBaseMessageResponse(&transport.BaseJSONRPCResponse{
		Jsonrpc: "2.0",
		Id:      id,
		Result:  json.RawMessage(`{"ok":true}`),
	})
}

func marshalRequestBody(id transport.RequestId) ([]byte, error) {
	return json.Marshal(&transport.BaseJSONRPCRequest{
		Jsonrpc: "2.0",
		Id:      id,
		Method:  "test/method",
		Params:  json.RawMessage(`{}`),
	})
}
