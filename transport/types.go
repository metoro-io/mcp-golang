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

// Custom Request unmarshaling
// Requires an Id, Jsonrpc and Method
func (m *BaseJSONRPCRequest) UnmarshalJSON(data []byte) error {
	required := struct {
		Id      *RequestId       `json:"id" yaml:"id" mapstructure:"id"`
		Jsonrpc *string          `json:"jsonrpc" yaml:"jsonrpc" mapstructure:"jsonrpc"`
		Method  *string          `json:"method" yaml:"method" mapstructure:"method"`
		Params  *json.RawMessage `json:"params" yaml:"params" mapstructure:"params"`
	}{}
	err := json.Unmarshal(data, &required)
	if err != nil {
		return err
	}
	if required.Id == nil {
		return errors.New("field id in BaseJSONRPCRequest: required")
	}
	if required.Jsonrpc == nil {
		return errors.New("field jsonrpc in BaseJSONRPCRequest: required")
	}
	if required.Method == nil {
		return errors.New("field method in BaseJSONRPCRequest: required")
	}
	if required.Params == nil {
		required.Params = new(json.RawMessage)
	}

	m.Id = *required.Id
	m.Jsonrpc = *required.Jsonrpc
	m.Method = *required.Method
	m.Params = *required.Params
	return nil
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

// Custom Notification unmarshaling
// Requires a Jsonrpc and Method
func (m *BaseJSONRPCNotification) UnmarshalJSON(data []byte) error {
	required := struct {
		Jsonrpc *string          `json:"jsonrpc" yaml:"jsonrpc" mapstructure:"jsonrpc"`
		Method  *string          `json:"method" yaml:"method" mapstructure:"method"`
		Id      *int64           `json:"id" yaml:"id" mapstructure:"id"`
		Params  *json.RawMessage `json:"params" yaml:"params" mapstructure:"params"`
	}{}
	err := json.Unmarshal(data, &required)
	if err != nil {
		return err
	}
	if required.Jsonrpc == nil {
		return errors.New("field jsonrpc in BaseJSONRPCNotification: required")
	}
	if required.Method == nil {
		return errors.New("field method in BaseJSONRPCNotification: required")
	}
	if required.Id != nil {
		return errors.New("field id in BaseJSONRPCNotification: not allowed")
	}
	if required.Params == nil {
		required.Params = new(json.RawMessage)
	}
	m.Jsonrpc = *required.Jsonrpc
	m.Method = *required.Method
	m.Params = *required.Params
	return nil
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

// Custom Response unmarshaling
// Requires an Id, Jsonrpc and Result
func (m *BaseJSONRPCResponse) UnmarshalJSON(data []byte) error {
	required := struct {
		Id      *RequestId       `json:"id" yaml:"id" mapstructure:"id"`
		Jsonrpc *string          `json:"jsonrpc" yaml:"jsonrpc" mapstructure:"jsonrpc"`
		Result  *json.RawMessage `json:"result" yaml:"result" mapstructure:"result"`
	}{}
	err := json.Unmarshal(data, &required)
	if err != nil {
		return err
	}
	if required.Id == nil {
		return errors.New("field id in BaseJSONRPCResponse: required")
	}
	if required.Jsonrpc == nil {
		return errors.New("field jsonrpc in BaseJSONRPCResponse: required")
	}
	if required.Result == nil {
		return errors.New("field result in BaseJSONRPCResponse: required")
	}
	m.Id = *required.Id
	m.Jsonrpc = *required.Jsonrpc
	m.Result = *required.Result

	return err
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
