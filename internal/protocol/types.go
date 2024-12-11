package protocol

type Result struct {
	// This result property is reserved by the protocol to allow clients and servers
	// to attach additional metadata to their responses.
	Meta ResultMeta `json:"_meta,omitempty" yaml:"_meta,omitempty" mapstructure:"_meta,omitempty"`

	AdditionalProperties interface{} `mapstructure:",remain"`
}

// This result property is reserved by the protocol to allow clients and servers to
// attach additional metadata to their responses.
type ResultMeta map[string]interface{}
