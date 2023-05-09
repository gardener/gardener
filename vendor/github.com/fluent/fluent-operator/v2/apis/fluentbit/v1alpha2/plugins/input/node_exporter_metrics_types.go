package input

import (
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// A plugin based on Prometheus Node Exporter to collect system / host level metrics. <br />
// **Note: Metrics collected with Node Exporter Metrics flow through a separate pipeline from logs and current filters do not operate on top of metrics.** <br />
// This plugin is currently only supported on Linux based operating systems. <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/inputs/node-exporter-metrics**
type NodeExporterMetrics struct {
	// Tag name associated to all records comming from this plugin.
	Tag string `json:"tag,omitempty"`
	// The rate at which metrics are collected from the host operating system, default is 5 seconds.
	ScrapeInterval string `json:"scrapeInterval,omitempty"`
	Path           *Path  `json:"path,omitempty"`
}

type Path struct {
	// The mount point used to collect process information and metrics.
	Procfs string `json:"procfs,omitempty"`
	// The path in the filesystem used to collect system metrics.
	Sysfs string `json:"sysfs,omitempty"`
}

func (_ *NodeExporterMetrics) Name() string {
	return "node_exporter_metrics"
}

// Params implement Section() method
func (d *NodeExporterMetrics) Params(_ plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	if d.Tag != "" {
		kvs.Insert("Tag", d.Tag)
	}
	if d.ScrapeInterval != "" {
		kvs.Insert("scrape_interval", d.ScrapeInterval)
	}
	if d.Path != nil {
		if d.Path.Procfs != "" {
			kvs.Insert("path.procfs", d.Path.Procfs)
		}
		if d.Path.Sysfs != "" {
			kvs.Insert("path.sysfs", d.Path.Sysfs)
		}
	}
	return kvs, nil
}
