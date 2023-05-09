package filter

import (
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// The Nest Filter plugin allows you to operate on or with nested data. Its modes of operation are "nest" and "lift". <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/filters/nest**
type Nest struct {
	plugins.CommonParams `json:",inline"`
	// Select the operation nest or lift
	// +kubebuilder:validation:Enum:=nest;lift
	Operation string `json:"operation,omitempty"`
	// Nest records which field matches the wildcard
	Wildcard []string `json:"wildcard,omitempty"`
	// Nest records matching the Wildcard under this key
	NestUnder string `json:"nestUnder,omitempty"`
	// Lift records nested under the Nested_under key
	NestedUnder string `json:"nestedUnder,omitempty"`
	// Prefix affected keys with this string
	AddPrefix string `json:"addPrefix,omitempty"`
	// Remove prefix from affected keys if it matches this string
	RemovePrefix string `json:"removePrefix,omitempty"`
}

func (_ *Nest) Name() string {
	return "nest"
}

func (n *Nest) Params(_ plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	err := n.AddCommonParams(kvs)
	if err != nil {
		return kvs, err
	}
	if n.Operation != "" {
		kvs.Insert("Operation", n.Operation)
	}
	for _, wc := range n.Wildcard {
		kvs.Insert("Wildcard", wc)
	}
	if n.NestUnder != "" {
		kvs.Insert("Nest_under", n.NestUnder)
	}
	if n.NestedUnder != "" {
		kvs.Insert("Nested_under", n.NestedUnder)
	}
	if n.AddPrefix != "" {
		kvs.Insert("Add_prefix", n.AddPrefix)
	}
	if n.RemovePrefix != "" {
		kvs.Insert("Remove_prefix", n.RemovePrefix)
	}
	return kvs, nil
}
