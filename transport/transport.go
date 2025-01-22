package transport

import (
	"context"
)

// Transport describes the minimal contract for a MCP transport that a client or server can communicate over.
type Transport interface {
	// Start starts processing messages on the transport, including any connection steps that might need to be taken.
	//
	// This method should only be called after callbacks are installed, or else messages may be lost.
	//
	// NOTE: This method should not be called explicitly when using Client, Server, or Protocol classes,
	// as they will implicitly call start().
	Start(ctx context.Context) error

	// Send sends a JSON-RPC message (request, notification or response).
	Send(ctx context.Context, message *BaseJsonRpcMessage) error

	// Close closes the connection.
	Close() error

	// SetCloseHandler sets the callback for when the connection is closed for any reason.
	// This should be invoked when Close() is called as well.
	SetCloseHandler(handler func())

	// SetErrorHandler sets the callback for when an error occurs.
	// Note that errors are not necessarily fatal; they are used for reporting any kind of exceptional condition out of band.
	SetErrorHandler(handler func(error))

	// SetMessageHandler sets the callback for when a message (request, notification or response) is received over the connection.
	// Partially deserializes the messages to pass a BaseJsonRpcMessage
	SetMessageHandler(handler func(ctx context.Context, message *BaseJsonRpcMessage))
}
