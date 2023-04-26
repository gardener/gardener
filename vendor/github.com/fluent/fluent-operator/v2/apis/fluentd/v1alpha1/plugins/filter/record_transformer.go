package filter

// The parameters inside <record> directives are considered to be new key-value pairs
type Record struct {
	// New field can be defined as key
	Key *string `json:"key"`
	// The value must from Record properties.
	// See https://docs.fluentd.org/filter/record_transformer#less-than-record-greater-than-directive
	Value *string `json:"value"`
}

// RecordTransformer defines the parameters for filter_record_transformer plugin
type RecordTransformer struct {
	Records []*Record `json:"records,omitempty"`
	// When set to true, the full Ruby syntax is enabled in the ${...} expression. The default value is false.
	// i.e: jsonized_record ${record.to_json}
	EnableRuby *bool `json:"enableRuby,omitempty"`
	// Automatically casts the field types. Default is false.
	// This option is effective only for field values comprised of a single placeholder.
	AutoTypeCast *bool `json:"autoTypecast,omitempty"`
	// By default, the record transformer filter mutates the incoming data. However, if this parameter is set to true, it modifies a new empty hash instead.
	RenewRecord *bool `json:"renewRecord,omitempty"`
	// renew_time_key foo overwrites the time of events with a value of the record field foo if exists. The value of foo must be a Unix timestamp.
	RenewTimeKey *string `json:"renewTimeKey,omitempty"`
	// A list of keys to keep. Only relevant if renew_record is set to true.
	KeepKeys *string `json:"keepKeys,omitempty"`
	// A list of keys to delete. Supports nested field via record_accessor syntax since v1.1.0.
	RemoveKeys *string `json:"removeKeys,omitempty"`
}
