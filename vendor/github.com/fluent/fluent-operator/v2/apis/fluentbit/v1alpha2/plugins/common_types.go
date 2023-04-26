package plugins

import (
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

type CommonParams struct {

	// Alias for the plugin
	Alias string `json:"alias,omitempty"`
	// RetryLimit describes how many times fluent-bit should retry to send data to a specific output. If set to false fluent-bit will try indefinetly. If set to any integer N>0 it will try at most N+1 times. Leading zeros are not allowed (values such as 007, 0150, 01 do not work). If this property is not defined fluent-bit will use the default value: 1.
	// +kubebuilder:validation:Pattern="^(((f|F)alse)|(no_limits)|(no_retries)|([1-9]+[0-9]*))$"
	RetryLimit string `json:"retryLimit,omitempty"`
}

func (c *CommonParams) AddCommonParams(kvs *params.KVs) error {
	if c.Alias != "" {
		kvs.Insert("Alias", c.Alias)
	}
	if c.RetryLimit != "" {
		kvs.Insert("Retry_Limit", c.RetryLimit)
	}
	return nil
}
