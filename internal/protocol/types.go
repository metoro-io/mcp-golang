package protocol

import (
	"encoding/json"
	"fmt"
	"github.com/metoro-io/mcp-golang/transport"
)

// A response to a request that indicates an error occurred.
type JSONRPCError struct {
	// Error corresponds to the JSON schema field "error".
	Error JSONRPCErrorError `json:"error" yaml:"error" mapstructure:"error"`

	// Id corresponds to the JSON schema field "id".
	Id transport.RequestId `json:"id" yaml:"id" mapstructure:"id"`

	// Jsonrpc corresponds to the JSON schema field "jsonrpc".
	Jsonrpc string `json:"jsonrpc" yaml:"jsonrpc" mapstructure:"jsonrpc"`
}

type JSONRPCErrorError struct {
	// The error type that occurred.
	Code int `json:"code" yaml:"code" mapstructure:"code"`

	// Additional information about the error. The value of this member is defined by
	// the sender (e.g. detailed error information, nested errors etc.).
	Data interface{} `json:"data,omitempty" yaml:"data,omitempty" mapstructure:"data,omitempty"`

	// A short description of the error. The message SHOULD be limited to a concise
	// single sentence.
	Message string `json:"message" yaml:"message" mapstructure:"message"`
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *JSONRPCErrorError) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if _, ok := raw["code"]; raw != nil && !ok {
		return fmt.Errorf("field code in JSONRPCErrorError: required")
	}
	if _, ok := raw["message"]; raw != nil && !ok {
		return fmt.Errorf("field message in JSONRPCErrorError: required")
	}
	type Plain JSONRPCErrorError
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	*j = JSONRPCErrorError(plain)
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *JSONRPCError) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if _, ok := raw["error"]; raw != nil && !ok {
		return fmt.Errorf("field error in JSONRPCError: required")
	}
	if _, ok := raw["id"]; raw != nil && !ok {
		return fmt.Errorf("field id in JSONRPCError: required")
	}
	if _, ok := raw["jsonrpc"]; raw != nil && !ok {
		return fmt.Errorf("field jsonrpc in JSONRPCError: required")
	}
	type Plain JSONRPCError
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	*j = JSONRPCError(plain)
	return nil
}

type Result struct {
	// This result property is reserved by the protocol to allow clients and servers
	// to attach additional metadata to their responses.
	Meta ResultMeta `json:"_meta,omitempty" yaml:"_meta,omitempty" mapstructure:"_meta,omitempty"`

	AdditionalProperties interface{} `mapstructure:",remain"`
}

// This result property is reserved by the protocol to allow clients and servers to
// attach additional metadata to their responses.
type ResultMeta map[string]interface{}
