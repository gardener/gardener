package parser

import (
	"fmt"

	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// The regex parser allows to define a custom Ruby Regular Expression that will use a named capture feature to define which content belongs to which key name. <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/parsers/regular-expression**
type Regex struct {
	Regex string `json:"regex,omitempty"`
	// Time_Key
	TimeKey string `json:"timeKey,omitempty"`
	// Time_Format, eg. %Y-%m-%dT%H:%M:%S %z
	TimeFormat string `json:"timeFormat,omitempty"`
	// Time_Keep
	TimeKeep *bool `json:"timeKeep,omitempty"`
	// Time_Offset, eg. +0200
	TimeOffset string `json:"timeOffset,omitempty"`
	Types      string `json:"types,omitempty"`
}

func (_ *Regex) Name() string {
	return "regex"
}

func (re *Regex) Params(_ plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	if re.Regex != "" {
		kvs.Insert("Regex", re.Regex)
	}
	if re.TimeKey != "" {
		kvs.Insert("Time_Key", re.TimeKey)
	}
	if re.TimeFormat != "" {
		kvs.Insert("Time_Format", re.TimeFormat)
	}
	if re.TimeKeep != nil {
		kvs.Insert("Time_Keep", fmt.Sprint(*re.TimeKeep))
	}
	if re.TimeOffset != "" {
		kvs.Insert("Time_Offset", re.TimeOffset)
	}
	if re.Types != "" {
		kvs.Insert("Types", re.Types)
	}
	return kvs, nil
}
