package output

import (
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// The stdout output plugin allows to print to the standard output the data received through the input plugin. <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/outputs/standard-output**
type Stdout struct {
	// Specify the data format to be printed. Supported formats are msgpack json, json_lines and json_stream.
	// +kubebuilder:validation:Enum:=msgpack;json;json_lines;json_stream
	Format string `json:"format,omitempty"`
	// Specify the name of the date field in output.
	JsonDateKey string `json:"jsonDateKey,omitempty"`
	// Specify the format of the date. Supported formats are double,  iso8601 (eg: 2018-05-30T09:39:52.000681Z) and epoch.
	// +kubebuilder:validation:Enum:= double;iso8601;epoch
	JsonDateFormat string `json:"jsonDateFormat,omitempty"`
}

func (_ *Stdout) Name() string {
	return "stdout"
}

// implement Section() method
func (s *Stdout) Params(_ plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	if s.Format != "" {
		kvs.Insert("Format", s.Format)
	}
	if s.JsonDateKey != "" {
		kvs.Insert("json_date_key", s.JsonDateKey)
	}
	if s.JsonDateFormat != "" {
		kvs.Insert("json_date_format", s.JsonDateFormat)
	}
	return kvs, nil
}
