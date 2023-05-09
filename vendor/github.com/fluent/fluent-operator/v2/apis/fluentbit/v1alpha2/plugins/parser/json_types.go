package parser

import (
	"fmt"

	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// The JSON parser plugin. <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/parsers/json**
type JSON struct {
	// Time_Key
	TimeKey string `json:"timeKey,omitempty"`
	// Time_Format, eg. %Y-%m-%dT%H:%M:%S %z
	TimeFormat string `json:"timeFormat,omitempty"`
	// Time_Keep
	TimeKeep *bool `json:"timeKeep,omitempty"`
}

func (_ *JSON) Name() string {
	return "json"
}

func (j *JSON) Params(_ plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	if j.TimeKey != "" {
		kvs.Insert("Time_Key", j.TimeKey)
	}
	if j.TimeFormat != "" {
		kvs.Insert("Time_Format", j.TimeFormat)
	}
	if j.TimeKeep != nil {
		kvs.Insert("Time_Keep", fmt.Sprint(*j.TimeKeep))
	}
	return kvs, nil
}
