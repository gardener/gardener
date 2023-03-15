package output

// Kafka2 defines the parameters for out_kafka output plugin
type Kafka2 struct {
	// The list of all seed brokers, with their host and port information. Default: localhost:9092
	Brokers *string `json:"brokers,omitempty"`
	// The field name for the target topic. If the field value is app, this plugin writes events to the app topic.
	TopicKey *string `json:"topicKey,omitempty"`
	// The name of the default topic. (default: nil)
	DefaultTopic *string `json:"defaultTopic,omitempty"`
	// Set fluentd event time to Kafka's CreateTime.
	UseEventTime *bool `json:"useEventTime,omitempty"`
	// The number of acks required per request.
	RequiredAcks *int16 `json:"requiredAcks,omitempty"`
	// The codec the producer uses to compress messages (default: nil).
	// +kubebuilder:validation:Enum:=gzip;snappy
	CompressionCodec *string `json:"compressionCodec,omitempty"`
}
