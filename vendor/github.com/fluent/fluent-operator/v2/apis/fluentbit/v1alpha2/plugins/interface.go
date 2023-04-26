package plugins

import "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"

// +kubebuilder:object:generate=false

// The Plugin interface defines methods for transferring input, filter
// and output plugins to textual section content.
type Plugin interface {
	Name() string
	Params(SecretLoader) (*params.KVs, error)
}
