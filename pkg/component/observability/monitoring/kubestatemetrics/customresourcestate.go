// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubestatemetrics

import (
	"fmt"
	"strings"

	"k8s.io/kube-state-metrics/v2/pkg/customresourcestate"
	"k8s.io/kube-state-metrics/v2/pkg/metric"
)

const (
	customResourceStateConfigMountDir      = "/config"
	customResourceStateConfigMountFile     = "custom-resource-state.yaml"
	customResourceStateConfigMapNamePrefix = "kube-state-metrics-custom-resource-state"
)

func newCustomResourceStateMetricNameForVPA(path, valuePath []string) string {
	metricName := "verticalpodautoscaler_" + strings.ToLower(strings.Join(path, "_"))
	if len(valuePath) > 0 {
		metricName += "_" + strings.ToLower(strings.Join(valuePath, "_"))
	}

	return metricName
}

func newCustomResourceStateGaugeMetricForVPA(path, valueFrom []string, help, unit string) customresourcestate.Generator {
	return customresourcestate.Generator{
		Name: newCustomResourceStateMetricNameForVPA(path, valueFrom),
		Help: help,
		Labels: customresourcestate.Labels{
			CommonLabels: map[string]string{
				"unit": unit,
			},
		},
		Each: customresourcestate.Metric{
			Type: metric.Gauge,
			Gauge: &customresourcestate.MetricGauge{
				MetricMeta: customresourcestate.MetricMeta{
					Path: path,
					LabelsFromPath: map[string][]string{
						"container": {"containerName"},
					},
				},
				ValueFrom: valueFrom,
				NilIsZero: true,
			},
		},
	}
}

func newCustomResourceStateMetricsForVPA() customresourcestate.Resource {
	resource := customresourcestate.Resource{
		GroupVersionKind: customresourcestate.GroupVersionKind{
			Group:   "autoscaling.k8s.io",
			Kind:    "VerticalPodAutoscaler",
			Version: "v1",
		},
		Labels: customresourcestate.Labels{
			LabelsFromPath: map[string][]string{
				"verticalpodautoscaler": {"metadata", "name"},
				"namespace":             {"metadata", "namespace"},
				"target_api_version":    {"spec", "targetRef", "apiVersion"},
				"target_kind":           {"spec", "targetRef", "kind"},
				"target_name":           {"spec", "targetRef", "name"},
			},
		},
	}

	units := map[string]string{
		"cpu":    "core",
		"memory": "byte",
	}

	helpMessages := map[string]string{
		"target":         "Target %s the VerticalPodAutoscaler recommends for the container.",
		"uncappedTarget": "Target %s the VerticalPodAutoscaler recommends for the container ignoring bounds.",
		"upperBound":     "Maximum %s the container can use before the VerticalPodAutoscaler updater evicts it.",
		"lowerBound":     "Minimum %s the container can use before the VerticalPodAutoscaler updater evicts it.",
		"minAllowed":     "Minimum %s the VerticalPodAutoscaler can set for containers matching the name.",
		"maxAllowed":     "Maximum %s the VerticalPodAutoscaler can set for containers matching the name.",
	}

	for _, res := range []string{"cpu", "memory"} {
		for _, attr := range []string{"target", "upperBound", "lowerBound", "uncappedTarget"} {
			generator := newCustomResourceStateGaugeMetricForVPA(
				[]string{"status", "recommendation", "containerRecommendations"},
				[]string{attr, res},
				fmt.Sprintf(helpMessages[attr], res),
				units[res],
			)

			resource.Metrics = append(resource.Metrics, generator)
		}

		for _, attr := range []string{"minAllowed", "maxAllowed"} {
			generator := newCustomResourceStateGaugeMetricForVPA(
				[]string{"spec", "resourcePolicy", "containerPolicies"},
				[]string{attr, res},
				fmt.Sprintf(helpMessages[attr], res),
				units[res],
			)

			resource.Metrics = append(resource.Metrics, generator)
		}
	}

	path := []string{"spec", "updatePolicy", "updateMode"}
	resource.Metrics = append(resource.Metrics, customresourcestate.Generator{
		Name: newCustomResourceStateMetricNameForVPA(path, nil),
		Help: "Update mode of the VerticalPodAutoscaler.",
		Each: customresourcestate.Metric{
			Type: metric.StateSet,
			StateSet: &customresourcestate.MetricStateSet{
				MetricMeta: customresourcestate.MetricMeta{
					Path: path,
				},
				LabelName: "update_mode",
				List:      []string{"Off", "Initial", "Recreate", "Auto"},
			},
		},
	})

	return resource
}

func newGardenCustomResourceStateMetrics() customresourcestate.Resource {
	gardenMetricNamePrefix := "garden"

	resource := customresourcestate.Resource{
		GroupVersionKind: customresourcestate.GroupVersionKind{
			Group:   "operator.gardener.cloud",
			Kind:    "Garden",
			Version: "v1alpha1",
		},
		MetricNamePrefix: &gardenMetricNamePrefix,
		Labels: customresourcestate.Labels{
			LabelsFromPath: map[string][]string{
				"name": {"metadata", "name"},
			},
		},
	}

	resource.Metrics = append(resource.Metrics, customresourcestate.Generator{
		Name: "garden_condition",
		Help: "represents a condition of a Garden object",
		Each: customresourcestate.Metric{
			Type: metric.StateSet,
			StateSet: &customresourcestate.MetricStateSet{
				LabelName: "status",
				List:      []string{"Progressing", "True", "False", "Unknown"},
				ValueFrom: []string{"status"},
				MetricMeta: customresourcestate.MetricMeta{
					LabelsFromPath: map[string][]string{
						"condition": {"type"},
					},
					Path: []string{"status", "conditions"},
				},
			},
		},
	})

	resource.Metrics = append(resource.Metrics, customresourcestate.Generator{
		Name: "garden_last_operation",
		Help: "denotes the last operation performed on a Garden object",
		Each: customresourcestate.Metric{
			Type: metric.StateSet,
			StateSet: &customresourcestate.MetricStateSet{
				LabelName: "last_operation",
				List:      []string{"Create", "Reconcile", "Delete", "Migrate", "Restore"},
				ValueFrom: []string{"type"},
				MetricMeta: customresourcestate.MetricMeta{
					Path: []string{"status", "lastOperation"},
				},
			},
		},
	})

	return resource
}

// Option is a functional option type used to configure the CustomResourceState settings
type Option func(*customresourcestate.Metrics)

// WithGardenResourceMetrics adds the custom resource state configuration for the Garden resource
func WithGardenResourceMetrics(c *customresourcestate.Metrics) {
	c.Spec.Resources = append(c.Spec.Resources, newGardenCustomResourceStateMetrics())
}

// WithVPAMetrics adds the custom resource state configuration for the VerticalPodAutoscaler resource
func WithVPAMetrics(c *customresourcestate.Metrics) {
	c.Spec.Resources = append(c.Spec.Resources, newCustomResourceStateMetricsForVPA())
}

// NewCustomResourceStateConfig returns a new CustomResourceState configuration that can be serialized
// and passed to the kube-state-metrics binary to create metrics from custom resource definitions
func NewCustomResourceStateConfig(options ...Option) customresourcestate.Metrics {
	metrics := customresourcestate.Metrics{
		Spec: customresourcestate.MetricsSpec{
			Resources: []customresourcestate.Resource{},
		},
	}

	for _, opt := range options {
		opt(&metrics)
	}

	return metrics
}
