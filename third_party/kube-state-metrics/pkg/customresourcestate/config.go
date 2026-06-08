// Copyright The Kubernetes Authors
// SPDX-License-Identifier: Apache-2.0

// Important:
// This file is copied from https://github.com/kubernetes/kube-state-metrics/blob/v2.19.0/pkg/customresourcestate/config.go.
// Only the struct/type definitions used by Gardener are included - runtime logic has been omitted.

package customresourcestate

import (
	"fmt"

	"github.com/gardener/gardener/third_party/kube-state-metrics/pkg/metric"
	"k8s.io/klog/v2"
)

// Metrics is the top level configuration object.
type Metrics struct {
	Spec MetricsSpec `yaml:"spec" json:"spec"`
}

// MetricsSpec is the configuration describing the custom resource state metrics to generate.
type MetricsSpec struct {
	// Resources is the list of custom resources to be monitored. A resource with the same GroupVersionKind may appear
	// multiple times (e.g., to customize the namespace or subsystem,) but will incur additional overhead.
	Resources []Resource `yaml:"resources" json:"resources"`
}

// Resource configures a custom resource for metric generation.
type Resource struct {
	// Labels are added to all metrics. If the same key is used in a metric, the value from the metric will overwrite the value here.
	Labels `yaml:",inline" json:",inline"`

	// MetricNamePrefix defines a prefix for all metrics of the resource.
	// If set to "", no prefix will be added.
	// Example: If set to "foo", MetricNamePrefix will be "foo_<metric>".
	MetricNamePrefix *string `yaml:"metricNamePrefix" json:"metricNamePrefix"`

	// GroupVersionKind of the custom resource to be monitored.
	GroupVersionKind GroupVersionKind `yaml:"groupVersionKind" json:"groupVersionKind"`

	// ResourcePlural sets the plural name of the resource. Defaults to the plural version of the Kind according to flect.Pluralize.
	ResourcePlural string `yaml:"resourcePlural" json:"resourcePlural"`

	// Metrics are the custom resource fields to be collected.
	Metrics []Generator `yaml:"metrics" json:"metrics"`
	// ErrorLogV defines the verbosity threshold for errors logged for this resource.
	ErrorLogV klog.Level `yaml:"errorLogV" json:"errorLogV"`
}

// GroupVersionKind is the Kubernetes group, version, and kind of a resource.
type GroupVersionKind struct {
	Group   string `yaml:"group" json:"group"`
	Version string `yaml:"version" json:"version"`
	Kind    string `yaml:"kind" json:"kind"`
}

func (gvk GroupVersionKind) String() string {
	return fmt.Sprintf("%s_%s_%s", gvk.Group, gvk.Version, gvk.Kind)
}

// Labels is common configuration of labels to add to metrics.
type Labels struct {
	// CommonLabels are added to all metrics.
	CommonLabels map[string]string `yaml:"commonLabels" json:"commonLabels"`
	// LabelsFromPath adds additional labels where the value is taken from a field in the resource.
	LabelsFromPath map[string][]string `yaml:"labelsFromPath" json:"labelsFromPath"`
}

// Generator describes a unique metric name.
type Generator struct {
	// Each targets a value or values from the resource.
	Each Metric `yaml:"each" json:"each"`

	// Labels are added to all metrics. Labels from Each will overwrite these if using the same key.
	Labels `yaml:",inline" json:",inline"`
	// Name of the metric. Subject to prefixing based on the configuration of the Resource.
	Name string `yaml:"name" json:"name"`
	// Help text for the metric.
	Help string `yaml:"help" json:"help"`
	// ErrorLogV defines the verbosity threshold for errors logged for this metric. Must be non-zero to override the resource setting.
	ErrorLogV klog.Level `yaml:"errorLogV" json:"errorLogV"`
}

// Metric defines a metric to expose.
// +union
type Metric struct {
	// Gauge defines a gauge metric.
	// +optional
	Gauge *MetricGauge `yaml:"gauge" json:"gauge"`
	// StateSet defines a state set metric.
	// +optional
	StateSet *MetricStateSet `yaml:"stateSet" json:"stateSet"`
	// Info defines an info metric.
	// +optional
	Info *MetricInfo `yaml:"info" json:"info"`
	// Type defines the type of the metric.
	// +unionDiscriminator
	Type metric.Type `yaml:"type" json:"type"`
}
