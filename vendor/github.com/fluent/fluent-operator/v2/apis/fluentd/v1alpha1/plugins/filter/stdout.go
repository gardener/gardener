package filter

import "github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins/common"

// Stdout defines the parameters for filter_stdout plugin
type Stdout struct {
	// The format section
	Format *common.Format `json:"format,omitempty"`
	// The inject section
	*common.Inject `json:"inject,omitempty"`
}
