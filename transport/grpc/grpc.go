package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/metoro-io/mcp-golang/transport"
	pb "github.com/metoro-io/mcp-golang/transport/grpc/protogen/jsonrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/structpb"
)

// GrpcServerTransport implements server-side transport for grpc communication
type GrpcServerTransport struct {
	pb.UnimplementedJSONRPCServiceServer
	mu        sync.Mutex
	onClose   func()
	onError   func(error)
	onMessage func(ctx context.Context, message *transport.BaseJsonRpcMessage)
	host      string
	port      int
	grpcOpts  []grpc.ServerOption
}

type GrpcServerTransportOption func(*GrpcServerTransport)

func WithHost(host string) GrpcServerTransportOption {
	return func(s *GrpcServerTransport) {
		s.host = host
	}
}

func WithPort(port int) GrpcServerTransportOption {
	return func(s *GrpcServerTransport) {
		s.port = port
	}
}

func WithGrpcOpts(opts ...grpc.ServerOption) GrpcServerTransportOption {
	return func(s *GrpcServerTransport) {
		s.grpcOpts = opts
	}
}

// NewGrpcServerTransport creates a new GRPC ServerTransport
func NewGrpcServerTransport(opts ...GrpcServerTransportOption) *GrpcServerTransport {
	srv := &GrpcServerTransport{
		port: 50051,
	}
	for _, opt := range opts {
		opt(srv)
	}
	return srv
}

type ctxKey string

func (t *GrpcServerTransport) Start(ctx context.Context) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", t.host, t.port))
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer(t.grpcOpts...)
	reflection.Register(grpcServer)

	pb.RegisterJSONRPCServiceServer(grpcServer, t)

	return grpcServer.Serve(lis)
}

func (t *GrpcServerTransport) Close() error {
	return nil
}

// Send sends a JSON-RPC message
func (t *GrpcServerTransport) Send(ctx context.Context, message *transport.BaseJsonRpcMessage) error {
	// TODO: debug log the response

	stream, ok := ctx.Value(ctxKey("stream")).(pb.JSONRPCService_TransportServer)
	if !ok || stream == nil {
		return fmt.Errorf("could not find a valid stream for sending response; ctx: %v", ctx)
	}

	msg, err := ToGenericRpcMessage(message)
	if err != nil {
		return fmt.Errorf("failed to convert BaseJsonRpcMessage to GenericRpcMessage; msg: %v; err: %v", message, err)
	}

	if err := stream.Send(msg); err != nil {
		return err
	}
	return nil
}

// SetCloseHandler sets the handler for close events
func (t *GrpcServerTransport) SetCloseHandler(handler func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onClose = handler
}

// SetErrorHandler sets the handler for error events
func (t *GrpcServerTransport) SetErrorHandler(handler func(error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onError = handler
}

// SetMessageHandler sets the handler for incoming messages
func (t *GrpcServerTransport) SetMessageHandler(handler func(ctx context.Context, message *transport.BaseJsonRpcMessage)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onMessage = handler
}

func (t *GrpcServerTransport) Transport(stream pb.JSONRPCService_TransportServer) error {
	fmt.Printf("transport started...\n")

	ctx := context.WithValue(context.Background(), ctxKey("stream"), stream)

	for {
		ms, err := stream.Recv()
		if err == io.EOF {
			fmt.Println("Stream closed by client")
			return nil
		}
		if err != nil {
			return err
		}
		fmt.Println("Received message...")

		if baseMsg, err := ToBaseJsonRpcMessage(ms); err != nil {
			return err
		} else {
			// TODO: debug log the received request
			t.onMessage(ctx, baseMsg)
		}
	}

	return nil
}

func ToBaseJsonRpcMessage(m *pb.GenericJSONRPCMessage) (*transport.BaseJsonRpcMessage, error) {
	msg := &transport.BaseJsonRpcMessage{}

	if tp, err := GetMessageType(m); err != nil {
		return nil, err
	} else {
		msg.Type = tp
	}

	var id transport.RequestId // TODO: only int64 is currently supported by metoro-io library

	switch m.TypedId.GetKind().(type) {
	case *pb.ID_Str:
		// TODO: support string ID processing
		return nil, fmt.Errorf("string type ID not supported yet: %v", m.TypedId.GetStr())
	case *pb.ID_Num:
		id = transport.RequestId(m.TypedId.GetNum())
	}

	switch msg.Type {
	case transport.BaseMessageTypeJSONRPCRequestType:
		params, err := m.Params.MarshalJSON()
		if err != nil {
			return nil, err
		}

		msg.JsonRpcRequest = &transport.BaseJSONRPCRequest{
			Jsonrpc: m.Jsonrpc,
			Id:      id,
			Method:  m.Method,
			Params:  params,
		}
	case transport.BaseMessageTypeJSONRPCNotificationType:
		params, err := m.Params.MarshalJSON()
		if err != nil {
			return nil, err
		}

		msg.JsonRpcNotification = &transport.BaseJSONRPCNotification{
			Jsonrpc: m.Jsonrpc,
			Method:  m.Method,
			Params:  params,
		}
	case transport.BaseMessageTypeJSONRPCResponseType:
		result, err := m.Result.MarshalJSON()
		if err != nil {
			return nil, err
		}
		msg.JsonRpcResponse = &transport.BaseJSONRPCResponse{
			Jsonrpc: m.Jsonrpc,
			Id:      id,
			Result:  result,
		}
	case transport.BaseMessageTypeJSONRPCErrorType:
		msg.JsonRpcError = &transport.BaseJSONRPCError{
			Jsonrpc: m.Jsonrpc,
			Id:      id,
			Error: transport.BaseJSONRPCErrorInner{
				Code:    int(m.Error.Code),
				Data:    m.Error.Data,
				Message: m.Error.Message,
			},
		}
	default:
		return nil, fmt.Errorf("unknown message type, couldn't marshal: %v", msg.Type)
	}

	return msg, nil
}

func GetMessageType(m *pb.GenericJSONRPCMessage) (transport.BaseMessageType, error) {
	// TODO: debug log the recevied response

	switch {
	case m.TypedId != nil && m.Method != "":
		return transport.BaseMessageTypeJSONRPCRequestType, nil
	case m.TypedId != nil && m.Result != nil:
		return transport.BaseMessageTypeJSONRPCResponseType, nil
	case m.TypedId != nil && m.Error != nil:
		return transport.BaseMessageTypeJSONRPCErrorType, nil
	case m.TypedId == nil && m.Method != "":
		return transport.BaseMessageTypeJSONRPCNotificationType, nil
	default:
		return "", fmt.Errorf("failed to find the type of the message")
	}
}

func ToGenericRpcMessage(m *transport.BaseJsonRpcMessage) (*pb.GenericJSONRPCMessage, error) {
	msg := &pb.GenericJSONRPCMessage{}

	switch m.Type {
	case transport.BaseMessageTypeJSONRPCRequestType:
		msg.Jsonrpc = m.JsonRpcRequest.Jsonrpc
		msg.TypedId = &pb.ID{Kind: &pb.ID_Num{Num: int64(m.JsonRpcRequest.Id)}}
		msg.Method = m.JsonRpcRequest.Method

		params, err := RawMessageToStruct(m.JsonRpcRequest.Params)
		if err != nil {
			return nil, err
		}
		msg.Params = params
	case transport.BaseMessageTypeJSONRPCResponseType:
		msg.Jsonrpc = m.JsonRpcResponse.Jsonrpc
		msg.TypedId = &pb.ID{Kind: &pb.ID_Num{Num: int64(m.JsonRpcResponse.Id)}}
		result, err := RawMessageToStruct(m.JsonRpcResponse.Result)
		if err != nil {
			return nil, err
		}
		msg.Result = result
	case transport.BaseMessageTypeJSONRPCNotificationType:
		msg.Jsonrpc = m.JsonRpcNotification.Jsonrpc
		msg.Method = m.JsonRpcNotification.Method
		params, err := RawMessageToStruct(m.JsonRpcNotification.Params)
		if err != nil {
			return nil, err
		}
		msg.Params = params
	case transport.BaseMessageTypeJSONRPCErrorType:
		msg.Jsonrpc = m.JsonRpcError.Jsonrpc
		msg.TypedId = &pb.ID{Kind: &pb.ID_Num{Num: int64(m.JsonRpcError.Id)}}
		var dataString string
		if m.JsonRpcError.Error.Data != nil {
			dataBytes, err := json.Marshal(m.JsonRpcError.Error.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal error data: %w", err)
			}
			dataString = string(dataBytes)
		}
		msg.Error = &pb.JSONRPCError{
			Data:    dataString,
			Code:    int32(m.JsonRpcError.Error.Code),
			Message: m.JsonRpcError.Error.Message,
		}
	default:
		return nil, fmt.Errorf("unsupported type for BaseJsonRpcMessage")
	}

	return msg, nil
}

func RawMessageToStruct(raw json.RawMessage) (*structpb.Struct, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return structpb.NewStruct(m)
}
