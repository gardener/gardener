package output

import (
	"fmt"

	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// Kafka output plugin allows to ingest your records into an Apache Kafka service. <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/outputs/kafka**
type Kafka struct {
	// Specify data format, options available: json, msgpack.
	Format string `json:"format,omitempty"`
	// Optional key to store the message
	MessageKey string `json:"messageKey,omitempty"`
	// If set, the value of Message_Key_Field in the record will indicate the message key.
	// If not set nor found in the record, Message_Key will be used (if set).
	MessageKeyField string `json:"messageKeyField,omitempty"`
	// Set the key to store the record timestamp
	TimestampKey string `json:"timestampKey,omitempty"`
	// iso8601 or double
	TimestampFormat string `json:"timestampFormat,omitempty"`
	// Single of multiple list of Kafka Brokers, e.g: 192.168.1.3:9092, 192.168.1.4:9092.
	Brokers string `json:"brokers,omitempty"`
	// Single entry or list of topics separated by comma (,) that Fluent Bit will use to send messages to Kafka.
	// If only one topic is set, that one will be used for all records.
	// Instead if multiple topics exists, the one set in the record by Topic_Key will be used.
	Topics string `json:"topics,omitempty"`
	// If multiple Topics exists, the value of Topic_Key in the record will indicate the topic to use.
	// E.g: if Topic_Key is router and the record is {"key1": 123, "router": "route_2"},
	// Fluent Bit will use topic route_2. Note that if the value of Topic_Key is not present in Topics,
	// then by default the first topic in the Topics list will indicate the topic to be used.
	TopicKey string `json:"topicKey,omitempty"`
	// {property} can be any librdkafka properties
	Rdkafka map[string]string `json:"rdkafka,omitempty"`
	//adds unknown topics (found in Topic_Key) to Topics. So in Topics only a default topic needs to be configured
	DynamicTopic *bool `json:"dynamicTopic,omitempty"`
	//Fluent Bit queues data into rdkafka library,
	//if for some reason the underlying library cannot flush the records the queue might fills up blocking new addition of records.
	//The queue_full_retries option set the number of local retries to enqueue the data.
	//The default value is 10 times, the interval between each retry is 1 second.
	//Setting the queue_full_retries value to 0 set's an unlimited number of retries.
	QueueFullRetries *int64 `json:"queueFullRetries,omitempty"`
}

func (*Kafka) Name() string {
	return "kafka"
}

// implement Section() method
func (k *Kafka) Params(_ plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	if k.Format != "" {
		kvs.Insert("Format", k.Format)
	}
	if k.MessageKey != "" {
		kvs.Insert("Message_Key", k.MessageKey)
	}
	if k.MessageKeyField != "" {
		kvs.Insert("Message_Key_Field", k.MessageKeyField)
	}
	if k.TimestampKey != "" {
		kvs.Insert("Timestamp_Key", k.TimestampKey)
	}
	if k.TimestampFormat != "" {
		kvs.Insert("Timestamp_Format", k.TimestampFormat)
	}
	if k.Brokers != "" {
		kvs.Insert("Brokers", k.Brokers)
	}
	if k.Topics != "" {
		kvs.Insert("Topics", k.Topics)
	}
	if k.TopicKey != "" {
		kvs.Insert("Topic_Key", k.TopicKey)
	}
	if k.DynamicTopic != nil {
		kvs.Insert("Dynamic_topic", fmt.Sprint(*k.DynamicTopic))
	}
	if k.QueueFullRetries != nil {
		kvs.Insert("queue_full_retries", fmt.Sprint(*k.QueueFullRetries))
	}

	kvs.InsertStringMap(k.Rdkafka, func(k, v string) (string, string) {
		return fmt.Sprintf("rdkafka.%s", k), v
	})

	return kvs, nil
}
