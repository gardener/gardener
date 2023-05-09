package input

import (
	"fmt"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// Fluent Bit exposes its own metrics to allow you to monitor the internals of your pipeline. <br />
// The collected metrics can be processed similarly to those from the Prometheus Node Exporter input plugin. <br />
// They can be sent to output plugins including Prometheus Exporter, Prometheus Remote Write or OpenTelemetry. <br />
// **Important note: Metrics collected with Node Exporter Metrics flow through a separate pipeline from logs and current filters do not operate on top of metrics.** <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/inputs/fluentbit-metrics**
type FluentbitMetrics struct {
	Tag string `json:"tag,omitempty"`

	// The rate at which metrics are collected from the host operating system. default is 2 seconds.
	ScrapeInterval string `json:"scrapeInterval,omitempty"`

	// Scrape metrics upon start, useful to avoid waiting for 'scrape_interval' for the first round of metrics.
	ScrapeOnStart *bool `json:"scrapeOnStart,omitempty"`
}

func (_ *FluentbitMetrics) Name() string {
	return "fluentbit_metrics"
}

func (f *FluentbitMetrics) Params(_ plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	if f.Tag != "" {
		kvs.Insert("Tag", f.Tag)
	}
	if f.ScrapeInterval != "" {
		kvs.Insert("scrape_interval", f.ScrapeInterval)
	}
	if f.ScrapeOnStart != nil {
		kvs.Insert("scrape_on_start", fmt.Sprint(*f.ScrapeOnStart))
	}
	return kvs, nil
}
