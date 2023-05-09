package output

import (
	"fmt"

	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// The tcp output plugin allows to send records to a remote TCP server. <br />
// The payload can be formatted in different ways as required. <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/outputs/tcp-and-tls**
type TCP struct {
	// Target host where Fluent-Bit or Fluentd are listening for Forward messages.
	Host string `json:"host,omitempty"`
	// TCP Port of the target service.
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=65535
	Port *int32 `json:"port,omitempty"`
	// Specify the data format to be printed. Supported formats are msgpack json, json_lines and json_stream.
	// +kubebuilder:validation:Enum:=msgpack;json;json_lines;json_stream
	Format string `json:"format,omitempty"`
	// TSpecify the name of the time key in the output record.
	// To disable the time key just set the value to false.
	JsonDateKey string `json:"jsonDateKey,omitempty"`
	// Specify the format of the date. Supported formats are double, epoch
	// and iso8601 (eg: 2018-05-30T09:39:52.000681Z)
	// +kubebuilder:validation:Enum:=double;epoch;iso8601
	JsonDateFormat string `json:"jsonDateFormat,omitempty"`
	*plugins.TLS   `json:"tls,omitempty"`
}

func (_ *TCP) Name() string {
	return "tcp"
}

func (t *TCP) Params(sl plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	if t.Host != "" {
		kvs.Insert("Host", t.Host)
	}
	if t.Port != nil {
		kvs.Insert("Port", fmt.Sprint(*t.Port))
	}
	if t.Format != "" {
		kvs.Insert("Format", t.Format)
	}
	if t.JsonDateKey != "" {
		kvs.Insert("json_date_key", t.JsonDateKey)
	}
	if t.JsonDateFormat != "" {
		kvs.Insert("json_date_format", t.JsonDateFormat)
	}
	if t.TLS != nil {
		tls, err := t.TLS.Params(sl)
		if err != nil {
			return nil, err
		}
		kvs.Merge(tls)
	}
	return kvs, nil
}
