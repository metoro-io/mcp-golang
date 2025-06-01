package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/metoro-io/mcp-golang/transport"
	pb "github.com/metoro-io/mcp-golang/transport/grpc/protogen/jsonrpc"
)

func TestNewGrpcServerTransport_Defaults(t *testing.T) {
	srv := NewGrpcServerTransport()
	if srv.port != 50051 {
		t.Errorf("expected default port 50051, got %d", srv.port)
	}
}

func TestWithHostAndPort(t *testing.T) {
	srv := NewGrpcServerTransport(WithHost("127.0.0.1"), WithPort(12345))
	if srv.host != "127.0.0.1" {
		t.Errorf("expected host '127.0.0.1', got '%s'", srv.host)
	}
	if srv.port != 12345 {
		t.Errorf("expected port 12345, got %d", srv.port)
	}
}

func TestSetCloseHandler(t *testing.T) {
	srv := NewGrpcServerTransport()
	called := false
	srv.SetCloseHandler(func() { called = true })
	if srv.onClose == nil {
		t.Error("onClose handler not set")
	}
	srv.onClose()
	if !called {
		t.Error("onClose handler not called")
	}
}

func TestSetErrorHandler(t *testing.T) {
	srv := NewGrpcServerTransport()
	var got error
	srv.SetErrorHandler(func(err error) { got = err })
	err := errors.New("test error")
	srv.onError(err)
	if got == nil || got.Error() != "test error" {
		t.Errorf("expected error 'test error', got '%v'", got)
	}
}

func TestSetMessageHandler(t *testing.T) {
	srv := NewGrpcServerTransport()
	called := false
	srv.SetMessageHandler(func(ctx context.Context, msg *transport.BaseJsonRpcMessage) { called = true })
	srv.onMessage(context.Background(), &transport.BaseJsonRpcMessage{})
	if !called {
		t.Error("onMessage handler not called")
	}
}

func TestToBaseJsonRpcMessage_UnsupportedStringID(t *testing.T) {
	msg := &pb.GenericJSONRPCMessage{
		TypedId: &pb.ID{Kind: &pb.ID_Str{Str: "abc"}},
		Method:  "testMethod",
	}
	_, err := ToBaseJsonRpcMessage(msg)
	if err == nil || err.Error() != "string type ID not supported yet: abc" {
		t.Errorf("expected string type ID error, got %v", err)
	}
}

func TestToBaseJsonRpcMessage_UnknownType(t *testing.T) {
	msg := &pb.GenericJSONRPCMessage{}
	_, err := ToBaseJsonRpcMessage(msg)
	if err == nil {
		t.Error("expected error for unknown message type")
	}
}
