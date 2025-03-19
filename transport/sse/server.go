package sse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/rvoh-emccaleb/mcp-golang/transport"
	"github.com/rvoh-emccaleb/mcp-golang/transport/base"
)

var (
	ErrConnectionRemoved = errors.New("sse connection removed")
)

// sseConnection represents a single SSE client sseConnection, and provides a way to send messages to the client.
type sseConnection struct {
	id      int64
	writer  http.ResponseWriter
	flusher http.Flusher
}

// ServerTransport implements server-side SSE transport
type ServerTransport struct {
	*base.Transport        // shared transport for handling all messages
	baseEndpoint    string // connection IDs are appended to this endpoint for SSE connections
	mu              sync.RWMutex
	sseConns        map[int64]*sseConnection // map of connection IDs to connections
	nextConnID      int64                    // atomic counter for generating connection IDs
	chunkSize       int                      // size of chunks for writing SSE messages
}

// Option is a function that configures a ServerTransport
type Option func(*ServerTransport)

// WithChunkSize sets the chunk size for writing SSE messages
func WithChunkSize(size int) Option {
	return func(t *ServerTransport) {
		if size > 0 {
			t.chunkSize = size
		}
	}
}

// NewServerTransport creates a new SSE server transport
func NewServerTransport(endpoint string, options ...Option) *ServerTransport {
	t := &ServerTransport{
		Transport:    base.NewTransport(),
		baseEndpoint: endpoint,
		sseConns:     make(map[int64]*sseConnection),
		chunkSize:    1024, // default to 1KB chunks
	}
	for _, opt := range options {
		opt(t)
	}
	return t
}

// HandleSSEConnInitialize handles a new SSE connection request
func (t *ServerTransport) HandleSSEConnInitialize(w http.ResponseWriter) (int64, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return 0, fmt.Errorf("streaming is not supported: provided http.ResponseWriter does not implement http.Flusher")
	}

	// Generate a unique endpoint URI for this connection
	connID := atomic.AddInt64(&t.nextConnID, 1)
	endpointURI := fmt.Sprintf("%s/%d", t.baseEndpoint, connID)

	sseConn := &sseConnection{
		id:      connID,
		writer:  w,
		flusher: flusher,
	}

	t.addSSEConnection(sseConn)

	// Set headers for SSE before writing any data
	h := sseConn.writer.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("Access-Control-Allow-Origin", "*")
	sseConn.writer.WriteHeader(http.StatusOK)

	// Send the initial endpoint event as required by the MCP specification
	err := t.writeEndpointEvent(sseConn, endpointURI)
	if err != nil {
		// We can't change the status code now, so bubble the error up, allowing request
		// handling to continue without sending a response to the client.
		return 0, err
	}

	return connID, nil
}

// writeEndpointEvent writes the endpoint URI over SSE
func (t *ServerTransport) writeEndpointEvent(sseConn *sseConnection, endpointURI string) error {
	_, err := sseConn.writer.Write(fmt.Appendf(nil, "event: endpoint\ndata: %s\n\n", endpointURI))
	if err != nil {
		t.RemoveSSEConnection(sseConn.id)
		return fmt.Errorf(
			"failed to write endpoint event for connection with id %d: %w. %w",
			sseConn.id,
			err,
			ErrConnectionRemoved,
		)
	}

	sseConn.flusher.Flush()

	return nil
}

// HandleMCPMessage handles incoming MCP (JSON-RPC) messages.
func (t *ServerTransport) HandleMCPMessage(w http.ResponseWriter, r *http.Request, connID int64) error {
	// First check if we have an active connection for this ID
	sseConn, ok := t.getSSEConnection(connID)
	if !ok {
		errMsg := fmt.Sprintf("no active connection found for id: %d", connID)
		http.Error(w, errMsg, http.StatusNotFound)
		return errors.New(errMsg)
	}

	if r.Header.Get("Content-Type") != "application/json" {
		errMsg := fmt.Sprintf("unsupported content type: %s", r.Header.Get("Content-Type"))
		http.Error(w, errMsg, http.StatusUnsupportedMediaType)
		return errors.New(errMsg)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		errMsg := "failed to read request body"
		http.Error(w, errMsg, http.StatusBadRequest)
		return fmt.Errorf("%s: %w", errMsg, err)
	}
	defer r.Body.Close()

	var receivedMsg transport.BaseJsonRpcMessage
	if err := json.Unmarshal(body, &receivedMsg); err != nil {
		errMsg := "failed to parse request body as a JSON-RPC message"
		http.Error(w, errMsg, http.StatusBadRequest)
		return fmt.Errorf("%s: %w", errMsg, err)
	}

	// Send 200 OK response back to the POST request after validating the message.
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte{}); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	// Note: Errors from this point on must be sent over the SSE connection.

	response, err := t.HandleMessage(r.Context(), body)
	if err != nil {
		switch receivedMsg.Type {
		case transport.BaseMessageTypeJSONRPCRequestType:
			newErr := t.writeMessageEvent(sseConn, transport.NewBaseMessageError(&transport.BaseJSONRPCError{
				Jsonrpc: "2.0",
				Id:      receivedMsg.JsonRpcRequest.Id,
				Error: transport.BaseJSONRPCErrorInner{
					Code:    -32603, // Internal error
					Message: fmt.Sprintf("error handling request: %v", err),
				},
			}))
			if newErr != nil {
				return fmt.Errorf("failed to write error sse message after encountering '%w' error when handling the request: %w", err, newErr)
			}

			return fmt.Errorf("error handling request: %w", err)

		case transport.BaseMessageTypeJSONRPCNotificationType:
			return fmt.Errorf("error handling notification: %w", err)

		default:
			return fmt.Errorf("error handling unknown message type %s: %w", receivedMsg.Type, err)
		}
	}

	// Only send response for requests (not for notifications)
	if receivedMsg.Type == transport.BaseMessageTypeJSONRPCRequestType {
		err := t.writeMessageEvent(sseConn, response)
		if err != nil {
			if errors.Is(err, ErrConnectionRemoved) {
				return err
			}

			// Can try to write an error message event, but if that fails, we're out of options.
			errorMsg := transport.NewBaseMessageError(&transport.BaseJSONRPCError{
				Jsonrpc: "2.0",
				Id:      receivedMsg.JsonRpcRequest.Id,
				Error: transport.BaseJSONRPCErrorInner{
					Code:    -32603, // Internal error
					Message: fmt.Sprintf("failed to write message event: %v", err),
				},
			})

			newErr := t.writeMessageEvent(sseConn, errorMsg)
			if newErr != nil {
				return fmt.Errorf("failed to write error message event after encountering '%w' error: %w", err, newErr)
			}

			return err
		}
	}

	return nil
}

// writeMessageEvent writes a JSON-RPC message over SSE
func (t *ServerTransport) writeMessageEvent(sseConn *sseConnection, message *transport.BaseJsonRpcMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Write the event header
	if _, err := io.WriteString(sseConn.writer, "event: message\ndata: "); err != nil {
		t.RemoveSSEConnection(sseConn.id)
		return fmt.Errorf(
			"failed to write message event for connection with id %d: %w. %w",
			sseConn.id,
			err,
			ErrConnectionRemoved,
		)
	}

	// Use the configured chunk size
	for i := 0; i < len(data); i += t.chunkSize {
		end := i + t.chunkSize
		if end > len(data) {
			end = len(data)
		}

		if _, err := sseConn.writer.Write(data[i:end]); err != nil {
			t.RemoveSSEConnection(sseConn.id)
			return fmt.Errorf(
				"failed to write message chunk for connection with id %d: %w. %w",
				sseConn.id,
				err,
				ErrConnectionRemoved,
			)
		}
		sseConn.flusher.Flush()
	}

	// Write the final newlines
	if _, err := io.WriteString(sseConn.writer, "\n\n"); err != nil {
		t.RemoveSSEConnection(sseConn.id)
		return fmt.Errorf(
			"failed to write message termination for connection with id %d: %w. %w",
			sseConn.id,
			err,
			ErrConnectionRemoved,
		)
	}

	sseConn.flusher.Flush()
	return nil
}

// Start implements Transport.Start
func (t *ServerTransport) Start(ctx context.Context) error {
	// Nothing to do here as connections are established via HandleSSERequest
	return nil
}

// Send implements Transport.Send.
func (t *ServerTransport) Send(ctx context.Context, message *transport.BaseJsonRpcMessage) error {
	key := message.JsonRpcResponse.Id
	responseChannel := t.ResponseMap[int64(key)]
	if responseChannel == nil {
		return fmt.Errorf("no response channel found for key: %d", key)
	}
	responseChannel <- message
	return nil
}

// Close implements Transport.Close
func (t *ServerTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Clear all SSE connections
	t.sseConns = make(map[int64]*sseConnection)

	// Close the base transport
	return t.Transport.Close()
}

// SetMessageHandler implements Transport.SetMessageHandler
func (t *ServerTransport) SetMessageHandler(handler func(ctx context.Context, message *transport.BaseJsonRpcMessage)) {
	t.Transport.SetMessageHandler(handler)
}

// SetErrorHandler implements Transport.SetErrorHandler
func (t *ServerTransport) SetErrorHandler(handler func(error)) {
	t.Transport.SetErrorHandler(handler)
}

// SetCloseHandler implements Transport.SetCloseHandler
func (t *ServerTransport) SetCloseHandler(handler func()) {
	t.Transport.SetCloseHandler(handler)
}

func (t *ServerTransport) addSSEConnection(sseConn *sseConnection) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sseConns[sseConn.id] = sseConn
}

func (t *ServerTransport) getSSEConnection(connID int64) (*sseConnection, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	sseConn, ok := t.sseConns[connID]
	return sseConn, ok
}

// RemoveSSEConnection removes an SSE connection from the transport
func (t *ServerTransport) RemoveSSEConnection(connID int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.sseConns, connID)
}
