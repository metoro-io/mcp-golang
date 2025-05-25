package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/metoro-io/mcp-golang/transport"
	mcphttp "github.com/metoro-io/mcp-golang/transport/http"
)

type ChiTransport struct {
	base *mcphttp.BaseTransport
}

func NewChiTransport(endpoint string) *ChiTransport {
	return &ChiTransport{
		base: mcphttp.NewBaseTransport(),
	}
}

// Start implements Transport.Start - no-op for chi transport as it's handled by chi
func (t *ChiTransport) Start(ctx context.Context) error {
	return nil
}

// Send implements Transport.Send
func (t *ChiTransport) Send(ctx context.Context, message *transport.BaseJsonRpcMessage) error {
	key := message.JsonRpcResponse.Id
	responseChannel := t.base.ResponseMap[int64(key)]
	if responseChannel == nil {
		return fmt.Errorf("no response channel found for key: %d", key)
	}
	responseChannel <- message
	return nil
}

func (t *ChiTransport) Close() error {
	if t.base.CloseHandler != nil {
		t.base.CloseHandler()
	}
	return nil
}

func (t *ChiTransport) Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is supported", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	body, err := t.base.ReadBody(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response, err := t.base.HandleMessage(ctx, body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		if t.base.ErrorHandler != nil {
			t.base.ErrorHandler(fmt.Errorf("failed to marshal response: %w", err))
		}
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)
}
