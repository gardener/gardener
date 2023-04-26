package filter

import "github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins/common"

// Parser defines the parameters for filter_parser plugin
type Parser struct {
	Parse *common.Parse `json:"parse"`

	// Specifies the field name in the record to parse. Required parameter.
	// i.e: If set keyName to log, {"key":"value","log":"{\"time\":1622473200,\"user\":1}"} => {"user":1}
	KeyName *string `json:"keyName"`
	// Keeps the original event time in the parsed result. Default is false.
	ReserveTime *bool `json:"reserveTime,omitempty"`
	// Keeps the original key-value pair in the parsed result. Default is false.
	// i.e: If set keyName to log, reverseData to true,
	// {"key":"value","log":"{\"user\":1,\"num\":2}"} => {"key":"value","log":"{\"user\":1,\"num\":2}","user":1,"num":2}
	ReserveData *bool `json:"reserveData,omitempty"`
	// Removes key_name field when parsing is succeeded.
	RemoveKeyNameField *bool `json:"removeKeyNameField,omitempty"`
	// If true, invalid string is replaced with safe characters and re-parse it.
	ReplaceInvalidSequence *bool `json:"replaceInvalidSequence,omitempty"`
	// Stores the parsed values with the specified key name prefix.
	InjectKeyPrefix *string `json:"injectKeyPrefix,omitempty"`
	// Stores the parsed values as a hash value in a field.
	HashValueField *string `json:"hashValueField,omitempty"`
	// Emits invalid record to @ERROR label. Invalid cases are: key does not exist;the format is not matched;an unexpected error.
	// If you want to ignore these errors, set false.
	EmitInvalidRecordToError *bool `json:"emitInvalidRecordToError,omitempty"`
}
