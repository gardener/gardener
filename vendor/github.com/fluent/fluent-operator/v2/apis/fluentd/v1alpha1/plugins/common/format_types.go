package common

import (
	"fmt"

	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins/params"
)

// FormatCommon defines common parameters of the format plugin
type FormatCommon struct {
	// The @id parameter specifies a unique name for the configuration.
	Id *string `json:"id,omitempty"`
	// The @type parameter specifies the type of the plugin.
	// +kubebuilder:validation:Enum:=out_file;json;ltsv;csv;msgpack;hash;single_value
	Type *string `json:"type,omitempty"`
	// The @log_level parameter specifies the plugin-specific logging level
	LogLevel *string `json:"logLevel,omitempty"`
}

// Format defines various parameters of the format plugin
type Format struct {
	FormatCommon `json:",inline,omitempty"`
	// Time defines time parameters for Format Plugins
	Time `json:",inline,omitempty"`
	// Delimiter for each field.
	Delimiter *string `json:"delimiter,omitempty"`
	// Output tag field if true.
	OutputTag *bool `json:"outputTag,omitempty"`
	// Output time field if true.
	OutputTime *bool `json:"outputTime,omitempty"`
	// Overwrites the default value in this plugin.
	TimeType *string `json:"timeType,omitempty"`
	// Overwrites the default value in this plugin.
	TimeFormat *string `json:"timeFormat,omitempty"`
	// Specify newline characters.
	// +kubebuilder:validation:Enum:=lf;crlf
	Newline *string `json:"newline,omitempty"`
}

func (f *Format) Name() string {
	return "format"
}

func (f *Format) Params(_ plugins.SecretLoader) (*params.PluginStore, error) {
	ps := params.NewPluginStore("format")
	if f.Id != nil {
		ps.InsertPairs("@id", fmt.Sprint(*f.Id))
	}
	if f.Type != nil {
		ps.InsertPairs("@type", fmt.Sprint(*f.Type))
	}
	if f.LogLevel != nil {
		ps.InsertPairs("@log_level", fmt.Sprint(*f.LogLevel))
	}
	if f.Delimiter != nil {
		ps.InsertPairs("delimiter", fmt.Sprint(*f.Delimiter))
	}
	if f.OutputTag != nil {
		ps.InsertPairs("output_tag", fmt.Sprint(*f.OutputTag))
	}
	if f.OutputTime != nil {
		ps.InsertPairs("output_time", fmt.Sprint(*f.OutputTime))
	}
	if f.TimeType != nil {
		ps.InsertPairs("time_type", fmt.Sprint(*f.TimeType))
	}
	if f.TimeFormat != nil {
		ps.InsertPairs("time_format", fmt.Sprint(*f.TimeFormat))
	}
	if f.Newline != nil {
		ps.InsertPairs("newline", fmt.Sprint(*f.Newline))
	}
	return ps, nil
}
