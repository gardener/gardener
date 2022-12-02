package filter

import (
	"github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// The Grep Filter plugin allows to match or exclude specific records based in regular expression patterns.
type Grep struct {
	plugins.CommonParams `json:",inline"`
	// Keep records which field matches the regular expression.
	// Value Format: FIELD REGEX
	Regex string `json:"regex,omitempty"`
	// Exclude records which field matches the regular expression.
	// Value Format: FIELD REGEX
	Exclude string `json:"exclude,omitempty"`
}

func (_ *Grep) Name() string {
	return "grep"
}

func (g *Grep) Params(_ plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	err := g.AddCommonParams(kvs)
	if err != nil {
		return kvs, err
	}
	if g.Regex != "" {
		kvs.Insert("Regex", g.Regex)
	}
	if g.Exclude != "" {
		kvs.Insert("Exclude", g.Exclude)
	}
	return kvs, nil
}
