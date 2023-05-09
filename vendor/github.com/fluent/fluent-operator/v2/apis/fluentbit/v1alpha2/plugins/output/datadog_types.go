package output

import (
	"fmt"

	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// DataDog output plugin allows you to ingest your logs into Datadog. <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/outputs/datadog**
type DataDog struct {
	// Host is the Datadog server where you are sending your logs.
	Host string `json:"host,omitempty"`
	// TLS controls whether to use end-to-end security communications security protocol.
	// Datadog recommends setting this to on.
	TLS *bool `json:"tls,omitempty"`
	// Compress  the payload in GZIP format.
	// Datadog supports and recommends setting this to gzip.
	Compress string `json:"compress,omitempty"`
	// Your Datadog API key.
	APIKey string `json:"apikey,omitempty"`
	// Specify an HTTP Proxy.
	Proxy string `json:"proxy,omitempty"`
	// To activate the remapping, specify configuration flag provider.
	Provider string `json:"provider,omitempty"`
	// Date key name for output.
	JSONDateKey string `json:"json_date_key,omitempty"`
	// If enabled, a tag is appended to output. The key name is used tag_key property.
	IncludeTagKey *bool `json:"include_tag_key,omitempty"`
	// The key name of tag. If include_tag_key is false, This property is ignored.
	TagKey string `json:"tag_key,omitempty"`
	// The human readable name for your service generating the logs.
	Service string `json:"dd_service,omitempty"`
	// A human readable name for the underlying technology of your service.
	Source string `json:"dd_source,omitempty"`
	// The tags you want to assign to your logs in Datadog.
	Tags string `json:"dd_tags,omitempty"`
	// By default, the plugin searches for the key 'log' and remap the value to the key 'message'. If the property is set, the plugin will search the property name key.
	MessageKey string `json:"dd_message_key,omitempty"`

	// *plugins.HTTP `json:"tls,omitempty"`
}

func (_ *DataDog) Name() string {
	return "datadog"
}

// implement Section() method
func (s *DataDog) Params(sl plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()

	if s.Host != "" {
		kvs.Insert("Host", s.Host)
	}
	if s.TLS != nil {
		kvs.Insert("TLS", fmt.Sprint(*s.TLS))
	}
	if s.Compress != "" {
		kvs.Insert("compress", s.Compress)
	}
	if s.APIKey != "" {
		kvs.Insert("apikey", s.APIKey)
	}
	if s.Proxy != "" {
		kvs.Insert("proxy", s.Proxy)
	}
	if s.Provider != "" {
		kvs.Insert("provider", s.Provider)
	}
	if s.JSONDateKey != "" {
		kvs.Insert("json_date_key", s.JSONDateKey)
	}
	if s.IncludeTagKey != nil {
		kvs.Insert("include_tag_key", fmt.Sprint(*s.IncludeTagKey))
	}
	if s.TagKey != "" {
		kvs.Insert("tag_key", s.TagKey)
	}
	if s.Service != "" {
		kvs.Insert("dd_service", s.Service)
	}
	if s.Source != "" {
		kvs.Insert("dd_source", s.Source)
	}
	if s.Tags != "" {
		kvs.Insert("dd_tags", s.Tags)
	}
	if s.MessageKey != "" {
		kvs.Insert("dd_message_key", s.MessageKey)
	}

	return kvs, nil
}
