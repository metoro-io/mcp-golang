package transport

import (
	"encoding/json"
	"errors"
)

type JSONRPCMessage interface{}

type RequestId int64

type BaseJSONRPCErrorInner struct {
	// The error type that occurred.
	Code int `json:"code" yaml:"code" mapstructure:"code"`

	// Additional information about the error. The value of this member is defined by
	// the sender (e.g. detailed error information, nested errors etc.).
	Data interface{} `json:"data,omitempty" yaml:"data,omitempty" mapstructure:"data,omitempty"`

	// A short description of the error. The message SHOULD be limited to a concise
	// single sentence.
	Message string `json:"message" yaml:"message" mapstructure:"message"`
}

// A response to a request that indicates an error occurred.
type BaseJSONRPCError struct {
	// Error corresponds to the JSON schema field "error".
	Error BaseJSONRPCErrorInner `json:"error" yaml:"error" mapstructure:"error"`

	// Id corresponds to the JSON schema field "id".
	Id RequestId `json:"id" yaml:"id" mapstructure:"id"`

	// Jsonrpc corresponds to the JSON schema field "jsonrpc".
	Jsonrpc string `json:"jsonrpc" yaml:"jsonrpc" mapstructure:"jsonrpc"`
}

type BaseJSONRPCRequest struct {
	// Id corresponds to the JSON schema field "id".
	Id RequestId `json:"id" yaml:"id" mapstructure:"id"`

	// Jsonrpc corresponds to the JSON schema field "jsonrpc".
	Jsonrpc string `json:"jsonrpc" yaml:"jsonrpc" mapstructure:"jsonrpc"`

	// Method corresponds to the JSON schema field "method".
	Method string `json:"method" yaml:"method" mapstructure:"method"`

	// Params corresponds to the JSON schema field "params".
	// It is stored as a []byte to enable efficient marshaling and unmarshaling into custom types later on in the protocol
	Params json.RawMessage `json:"params,omitempty" yaml:"params,omitempty" mapstructure:"params,omitempty"`
}

type BaseJSONRPCNotification struct {
	// Jsonrpc corresponds to the JSON schema field "jsonrpc".
	Jsonrpc string `json:"jsonrpc" yaml:"jsonrpc" mapstructure:"jsonrpc"`

	// Method corresponds to the JSON schema field "method".
	Method string `json:"method" yaml:"method" mapstructure:"method"`

	// Params corresponds to the JSON schema field "params".
	// It is stored as a []byte to enable efficient marshaling and unmarshaling into custom types later on in the protocol
	Params json.RawMessage `json:"params,omitempty" yaml:"params,omitempty" mapstructure:"params,omitempty"`
}

type JsonRpcBody interface{}

type BaseJSONRPCResponse struct {
	// Id corresponds to the JSON schema field "id".
	Id RequestId `json:"id" yaml:"id" mapstructure:"id"`

	// Jsonrpc corresponds to the JSON schema field "jsonrpc".
	Jsonrpc string `json:"jsonrpc" yaml:"jsonrpc" mapstructure:"jsonrpc"`

	// Result corresponds to the JSON schema field "result".
	Result json.RawMessage `json:"result" yaml:"result" mapstructure:"result"`
}

type BaseMessageType string

const (
	BaseMessageTypeJSONRPCRequestType      BaseMessageType = "request"
	BaseMessageTypeJSONRPCNotificationType BaseMessageType = "notification"
	BaseMessageTypeJSONRPCResponseType     BaseMessageType = "response"
	BaseMessageTypeJSONRPCErrorType        BaseMessageType = "error"
)

type BaseJsonRpcMessage struct {
	Type                BaseMessageType
	JsonRpcRequest      *BaseJSONRPCRequest
	JsonRpcNotification *BaseJSONRPCNotification
	JsonRpcResponse     *BaseJSONRPCResponse
	JsonRpcError        *BaseJSONRPCError
}

func (m *BaseJsonRpcMessage) MarshalJSON() ([]byte, error) {
	switch m.Type {
	case BaseMessageTypeJSONRPCRequestType:
		return json.Marshal(m.JsonRpcRequest)
	case BaseMessageTypeJSONRPCNotificationType:
		return json.Marshal(m.JsonRpcNotification)
	case BaseMessageTypeJSONRPCResponseType:
		return json.Marshal(m.JsonRpcResponse)
	case BaseMessageTypeJSONRPCErrorType:
		return json.Marshal(m.JsonRpcError)
	default:
		return nil, errors.New("unknown message type, couldn't marshal")
	}
}

func NewBaseMessageNotification(notification *BaseJSONRPCNotification) *BaseJsonRpcMessage {
	return &BaseJsonRpcMessage{
		Type:                BaseMessageTypeJSONRPCNotificationType,
		JsonRpcNotification: notification,
	}
}

func NewBaseMessageRequest(request *BaseJSONRPCRequest) *BaseJsonRpcMessage {
	return &BaseJsonRpcMessage{
		Type:           BaseMessageTypeJSONRPCRequestType,
		JsonRpcRequest: request,
	}
}

func NewBaseMessageResponse(response *BaseJSONRPCResponse) *BaseJsonRpcMessage {
	return &BaseJsonRpcMessage{
		Type:            BaseMessageTypeJSONRPCResponseType,
		JsonRpcResponse: response,
	}
}

func NewBaseMessageError(error *BaseJSONRPCError) *BaseJsonRpcMessage {
	return &BaseJsonRpcMessage{
		Type: BaseMessageTypeJSONRPCErrorType,
		JsonRpcError: &BaseJSONRPCError{
			Error:   error.Error,
			Id:      error.Id,
			Jsonrpc: error.Jsonrpc,
		},
	}
}
