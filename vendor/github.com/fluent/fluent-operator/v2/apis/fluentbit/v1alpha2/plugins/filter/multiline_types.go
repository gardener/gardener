package filter

import (
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// The Multiline Filter helps to concatenate messages that originally belong to one context but were split across multiple records or log lines. <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/filters/multiline-stacktrace**
type Multiline struct {
	plugins.CommonParams `json:",inline"`
	// The Inline struct helps to concatenate messages that originally belong to one context but were split across multiple records or log lines.
	*Multi `json:",inline"`
}

type Multi struct {
	// Specify one or multiple Multiline Parsing definitions to apply to the content.
	//You can specify multiple multiline parsers to detect different formats by separating them with a comma.
	Parser string `json:"parser"`
	//Key name that holds the content to process.
	//Note that a Multiline Parser definition can already specify the key_content to use, but this option allows to overwrite that value for the purpose of the filter.
	KeyContent string `json:"keyContent,omitempty"`
}

func (_ *Multiline) Name() string {
	return "multiline"
}

func (m *Multiline) Params(_ plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	err := m.AddCommonParams(kvs)
	if err != nil {
		return kvs, err
	}
	if m.Multi != nil {
		if m.Multi.Parser != "" {
			kvs.Insert("multiline.parser", m.Multi.Parser)
		}
		if m.Multi.KeyContent != "" {
			kvs.Insert("multiline.key_content", m.Multi.KeyContent)
		}
	}
	return kvs, nil
}
