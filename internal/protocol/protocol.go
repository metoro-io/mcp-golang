// This file implements the core protocol layer for JSON-RPC communication in the MCP SDK.
// It handles the protocol-level concerns of JSON-RPC messaging, including request/response
// correlation, progress tracking, request cancellation, and error handling.
//
// Key Components:
//
// 1. Protocol:
//   - Core type managing JSON-RPC communication
//   - Handles message correlation and lifecycle
//   - Supports:
//   - Request/Response with timeouts
//   - Notifications (one-way messages)
//   - Progress updates during long operations
//   - Request cancellation
//   - Error propagation
//
// 2. Request Handling:
//   - Automatic request ID generation
//   - Context-based cancellation
//   - Configurable timeouts
//   - Progress callback support
//   - Response correlation using channels
//
// 3. Message Types:
//   - JSONRPCRequest: Outgoing requests with IDs
//   - JSONRPCNotification: One-way messages
//   - JSONRPCError: Error responses
//   - Progress: Updates during long operations
//
// 4. Handler Registration:
//   - Request handlers for method calls
//   - Notification handlers for events
//   - Progress handlers for long operations
//   - Error handlers for protocol errors
//
// Thread Safety:
//   - All public methods are thread-safe
//   - Uses sync.RWMutex for state protection
//   - Safe for concurrent requests and handlers
//
// Usage:
//
//	transport := NewStdioTransport()
//	protocol := NewProtocol(transport)
//
//	// Start protocol
//	protocol.Connect(transport)
//	defer protocol.Close()
//
//	// Make a request
//	ctx := context.Background()
//	response, err := protocol.Request(ctx, "method", params, &RequestOptions{
//	    Timeout: 5 * time.Second,
//	    OnProgress: func(p Progress) {
//	        // Handle progress updates
//	    },
//	})
//
// Error Handling:
//   - Context-based cancellation
//   - Timeout management
//   - Proper cleanup of pending requests
//   - Detailed error information
//
// For more details, see the test file protocol_test.go.
package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/metoro-io/mcp-golang/transport"
	"sync"
	"time"
)

const DefaultRequestTimeoutMsec = 60000

// Progress represents a progress update
type Progress struct {
	Progress int64 `json:"progress"`
	Total    int64 `json:"total"`
}

// ProgressCallback is a callback for progress notifications
type ProgressCallback func(progress Progress)

// ProtocolOptions contains additional initialization options
type ProtocolOptions struct {
	// Whether to restrict emitted requests to only those that the remote side has indicated
	// that they can handle, through their advertised capabilities.
	EnforceStrictCapabilities bool
}

// RequestOptions contains options that can be given per request
type RequestOptions struct {
	// OnProgress is called when progress notifications are received from the remote end
	OnProgress ProgressCallback
	// Context can be used to cancel an in-flight request
	Context context.Context
	// Timeout specifies a timeout for this request. If exceeded, an error with code
	// RequestTimeout will be returned. If not specified, DefaultRequestTimeoutMsec will be used
	Timeout time.Duration
}

// RequestHandlerExtra contains extra data given to request handlers
type RequestHandlerExtra struct {
	// Context used to communicate if the request was cancelled from the sender's side
	Context context.Context
}

// Protocol implements MCP protocol framing on top of a pluggable transport,
// including features like request/response linking, notifications, and progress
type Protocol struct {
	transport transport.Transport
	options   *ProtocolOptions

	requestMessageID int64
	mu               sync.RWMutex

	// Maps method name to request handler
	requestHandlers map[string]func(*transport.BaseJSONRPCRequest, RequestHandlerExtra) (interface{}, error) // Result or error
	// Maps request ID to cancellation function
	requestCancellers map[transport.RequestId]context.CancelFunc
	// Maps method name to notification handler
	notificationHandlers map[string]func(notification *transport.BaseJSONRPCNotification) error
	// Maps message ID to response handler
	responseHandlers map[int64]chan *responseEnvelope
	// Maps message ID to progress handler
	progressHandlers map[int64]ProgressCallback

	// Callback for when the connection is closed for any reason
	OnClose func()
	// Callback for when an error occurs
	OnError func(error)
	// Handler to invoke for any request types that do not have their own handler installed
	FallbackRequestHandler func(request *transport.BaseJSONRPCRequest) (interface{}, error)
	// Handler to invoke for any notification types that do not have their own handler installed
	FallbackNotificationHandler func(notification *transport.BaseJSONRPCNotification) error
}

type responseEnvelope struct {
	response interface{}
	err      error
}

// NewProtocol creates a new Protocol instance
func NewProtocol(options *ProtocolOptions) *Protocol {
	p := &Protocol{
		options:              options,
		requestHandlers:      make(map[string]func(*transport.BaseJSONRPCRequest, RequestHandlerExtra) (interface{}, error)),
		requestCancellers:    make(map[transport.RequestId]context.CancelFunc),
		notificationHandlers: make(map[string]func(*transport.BaseJSONRPCNotification) error),
		responseHandlers:     make(map[int64]chan *responseEnvelope),
		progressHandlers:     make(map[int64]ProgressCallback),
	}

	// Set up default handlers
	p.SetNotificationHandler("notifications/cancelled", p.handleCancelledNotification)
	p.SetNotificationHandler("$/progress", p.handleProgressNotification)
	p.SetRequestHandler("ping", func(req *transport.BaseJSONRPCRequest, _ RequestHandlerExtra) (interface{}, error) {
		return Result{}, nil
	})

	return p
}

// Connect attaches to the given transport, starts it, and starts listening for messages
func (p *Protocol) Connect(tr transport.Transport) error {
	p.transport = tr

	tr.SetCloseHandler(func() {
		p.handleClose()
	})

	tr.SetErrorHandler(func(err error) {
		p.handleError(err)
	})

	tr.SetMessageHandler(func(message *transport.BaseJsonRpcMessage) {
		switch m := message.Type; {
		case m == transport.BaseMessageTypeJSONRPCRequestType:
			p.handleRequest(message.JsonRpcRequest)
		case m == transport.BaseMessageTypeJSONRPCNotificationType:
			p.handleNotification(message.JsonRpcNotification)
		}
	})

	return tr.Start(context.Background())
}

func (p *Protocol) handleClose() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Clear all handlers
	p.requestHandlers = make(map[string]func(*transport.BaseJSONRPCRequest, RequestHandlerExtra) (interface{}, error))
	p.notificationHandlers = make(map[string]func(notification *transport.BaseJSONRPCNotification) error)

	// Cancel all pending requests
	for _, cancel := range p.requestCancellers {
		cancel()
	}
	p.requestCancellers = make(map[transport.RequestId]context.CancelFunc)

	// Close all response channels with error
	for id, ch := range p.responseHandlers {
		ch <- &responseEnvelope{err: fmt.Errorf("connection closed")}
		close(ch)
		delete(p.responseHandlers, id)
	}

	p.progressHandlers = make(map[int64]ProgressCallback)

	if p.OnClose != nil {
		p.OnClose()
	}
}

func (p *Protocol) handleError(err error) {
	if p.OnError != nil {
		p.OnError(err)
	}
}

func (p *Protocol) handleNotification(notification *transport.BaseJSONRPCNotification) {
	p.mu.RLock()
	handler := p.notificationHandlers[notification.Method]
	if handler == nil {
		handler = p.FallbackNotificationHandler
	}
	p.mu.RUnlock()

	if handler == nil {
		return
	}

	go func() {
		if err := handler(notification); err != nil {
			p.handleError(fmt.Errorf("notification handler error: %w", err))
		}
	}()
}

func (p *Protocol) handleRequest(request *transport.BaseJSONRPCRequest) {
	p.mu.RLock()
	handler := p.requestHandlers[request.Method]
	if handler == nil {
		handler = func(req *transport.BaseJSONRPCRequest, extra RequestHandlerExtra) (interface{}, error) {
			if p.FallbackRequestHandler != nil {
				return p.FallbackRequestHandler(req)
			}
			return Result{}, fmt.Errorf("method not found: %s", req.Method)
		}
	}
	p.mu.RUnlock()

	ctx, cancel := context.WithCancel(context.Background())
	p.mu.Lock()
	p.requestCancellers[request.Id] = cancel
	p.mu.Unlock()

	go func() {
		defer func() {
			p.mu.Lock()
			delete(p.requestCancellers, request.Id)
			p.mu.Unlock()
			cancel()
		}()

		result, err := handler(request, RequestHandlerExtra{Context: ctx})
		if err != nil {
			p.sendErrorResponse(request.Id, err)
			return
		}

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      request.Id,
			"result":  result,
		}

		if err := p.transport.Send(response); err != nil {
			p.handleError(fmt.Errorf("failed to send response: %w", err))
		}
	}()
}

func (p *Protocol) handleProgressNotification(notification *transport.BaseJSONRPCNotification) error {
	var params struct {
		Progress      int64 `json:"progress"`
		Total         int64 `json:"total"`
		ProgressToken int64 `json:"progressToken"`
	}

	if err := json.Unmarshal(notification.Params, &params); err != nil {
		return fmt.Errorf("failed to unmarshal progress params: %w", err)
	}

	p.mu.RLock()
	handler := p.progressHandlers[params.ProgressToken]
	p.mu.RUnlock()

	if handler != nil {
		handler(Progress{
			Progress: params.Progress,
			Total:    params.Total,
		})
	}

	return nil
}

func (p *Protocol) handleCancelledNotification(notification *transport.BaseJSONRPCNotification) error {
	var params struct {
		RequestId transport.RequestId `json:"requestId"`
		Reason    string              `json:"reason"`
	}

	if err := json.Unmarshal(notification.Params, &params); err != nil {
		return fmt.Errorf("failed to unmarshal cancelled params: %w", err)
	}

	p.mu.RLock()
	cancel := p.requestCancellers[params.RequestId]
	p.mu.RUnlock()

	if cancel != nil {
		cancel()
	}

	return nil
}

func (p *Protocol) handleResponse(response interface{}, errResp *JSONRPCError) {
	var id int64
	var result interface{}
	var err error

	if errResp != nil {
		id = int64(errResp.Id)
		err = fmt.Errorf("RPC error %d: %s", errResp.Error.Code, errResp.Error.Message)
	} else {
		// Parse the response
		resp := response.(map[string]interface{})
		switch v := resp["id"].(type) {
		case float64:
			id = int64(v)
		case int64:
			id = v
		case int:
			id = int64(v)
		default:
			p.handleError(fmt.Errorf("unexpected id type: %T", resp["id"]))
			return
		}
		result = resp["result"]
	}

	p.mu.RLock()
	ch := p.responseHandlers[id]
	p.mu.RUnlock()

	if ch != nil {
		ch <- &responseEnvelope{
			response: result,
			err:      err,
		}
	}
}

// Close closes the connection
func (p *Protocol) Close() error {
	if p.transport != nil {
		return p.transport.Close()
	}
	return nil
}

// Request sends a request and waits for a response
func (p *Protocol) Request(ctx context.Context, method string, params interface{}, opts *RequestOptions) (interface{}, error) {
	if p.transport == nil {
		return nil, fmt.Errorf("not connected")
	}

	if opts == nil {
		opts = &RequestOptions{}
	}

	if opts.Context == nil {
		opts.Context = ctx
	}

	if opts.Timeout == 0 {
		opts.Timeout = time.Duration(DefaultRequestTimeoutMsec) * time.Millisecond
	}

	p.mu.Lock()
	id := p.requestMessageID
	p.requestMessageID++
	ch := make(chan *responseEnvelope, 1)
	p.responseHandlers[id] = ch
	if opts.OnProgress != nil {
		p.progressHandlers[id] = opts.OnProgress
	}
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		delete(p.responseHandlers, id)
		delete(p.progressHandlers, id)
		p.mu.Unlock()
	}()

	// Create request with meta information if needed
	requestParams := params
	if opts.OnProgress != nil {
		meta := map[string]interface{}{
			"progressToken": id,
		}
		if params == nil {
			requestParams = map[string]interface{}{
				"_meta": meta,
			}
		} else if paramsMap, ok := params.(map[string]interface{}); ok {
			paramsMap["_meta"] = meta
			requestParams = paramsMap
		} else {
			return nil, fmt.Errorf("params must be nil or map[string]interface{} when using progress")
		}
	}

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  requestParams,
		"id":      id,
	}

	if err := p.transport.Send(request); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	select {
	case envelope := <-ch:
		if envelope.err != nil {
			return nil, envelope.err
		}
		return envelope.response, nil
	case <-opts.Context.Done():
		p.sendCancelNotification(id, opts.Context.Err().Error())
		return nil, opts.Context.Err()
	case <-time.After(opts.Timeout):
		p.sendCancelNotification(id, "request timeout")
		return nil, fmt.Errorf("request timeout after %v", opts.Timeout)
	}
}

func (p *Protocol) sendCancelNotification(requestID int64, reason string) {
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/cancelled",
		"params": map[string]interface{}{
			"requestId": requestID,
			"reason":    reason,
		},
	}

	if err := p.transport.Send(notification); err != nil {
		p.handleError(fmt.Errorf("failed to send cancel notification: %w", err))
	}
}

func (p *Protocol) sendErrorResponse(requestID transport.RequestId, err error) {
	response := JSONRPCError{
		Jsonrpc: "2.0",
		Id:      requestID,
		Error: JSONRPCErrorError{
			Code:    -32000, // Internal error
			Message: err.Error(),
		},
	}

	if err := p.transport.Send(response); err != nil {
		p.handleError(fmt.Errorf("failed to send error response: %w", err))
	}
}

// Notification emits a notification, which is a one-way message that does not expect a response
func (p *Protocol) Notification(method string, params interface{}) error {
	if p.transport == nil {
		return fmt.Errorf("not connected")
	}

	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}

	return p.transport.Send(notification)
}

// SetRequestHandler registers a handler to invoke when this protocol object receives a request with the given method
func (p *Protocol) SetRequestHandler(method string, handler func(*transport.BaseJSONRPCRequest, RequestHandlerExtra) (interface{}, error)) {
	p.mu.Lock()
	p.requestHandlers[method] = handler
	p.mu.Unlock()
}

// RemoveRequestHandler removes the request handler for the given method
func (p *Protocol) RemoveRequestHandler(method string) {
	p.mu.Lock()
	delete(p.requestHandlers, method)
	p.mu.Unlock()
}

// SetNotificationHandler registers a handler to invoke when this protocol object receives a notification with the given method
func (p *Protocol) SetNotificationHandler(method string, handler func(notification *transport.BaseJSONRPCNotification) error) {
	p.mu.Lock()
	p.notificationHandlers[method] = handler
	p.mu.Unlock()
}

// RemoveNotificationHandler removes the notification handler for the given method
func (p *Protocol) RemoveNotificationHandler(method string) {
	p.mu.Lock()
	delete(p.notificationHandlers, method)
	p.mu.Unlock()
}
