// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// CentralPodMonitors returns the central PodMonitor resources for the seed prometheus.
func CentralPodMonitors() []*monitoringv1.PodMonitor {
	return []*monitoringv1.PodMonitor{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "extensions",
			},
			Spec: monitoringv1.PodMonitorSpec{
				// Selector: metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				// 	{Key: "prometheus.io/scrape", Values: []string{"true"}, Operator: metav1.LabelSelectorOpIn},
				// 	{Key: "prometheus.io/port", Operator: metav1.LabelSelectorOpExists},
				// }},
				NamespaceSelector: monitoringv1.NamespaceSelector{Any: true},
				PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{
					RelabelConfigs: []monitoringv1.RelabelConfig{
						// TODO: These annotations should actually be labels so that PodMonitorSpec.Selector can be used
						//  instead of manually crafting this relabel config.
						{
							SourceLabels: []monitoringv1.LabelName{
								"__meta_kubernetes_namespace",
								"__meta_kubernetes_pod_annotation_prometheus_io_scrape",
								"__meta_kubernetes_pod_annotation_prometheus_io_port",
							},
							Regex:  `extension-(.+);true;(.+)`,
							Action: "keep",
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_annotation_prometheus_io_name"},
							Regex:        `(.+)`,
							Action:       "replace",
							TargetLabel:  "job",
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__address__", "__meta_kubernetes_pod_annotation_prometheus_io_port"},
							Regex:        `([^:]+)(?::\d+)?;(\d+)`,
							Action:       "replace",
							Replacement:  ptr.To(`$1:$2`),
							TargetLabel:  "__address__",
						},
						{
							Action: "labelmap",
							Regex:  `__meta_kubernetes_pod_label_(.+)`,
						},
					},
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "garden",
			},
			Spec: monitoringv1.PodMonitorSpec{
				// Selector: metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				// 	{Key: "prometheus.io/scrape", Values: []string{"true"}, Operator: metav1.LabelSelectorOpIn},
				// 	{Key: "prometheus.io/port", Operator: metav1.LabelSelectorOpExists},
				// }},
				PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{
					Scheme:    "https",
					TLSConfig: &monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)},
					RelabelConfigs: []monitoringv1.RelabelConfig{
						// TODO: These annotations should actually be labels so that PodMonitorSpec.Selector can be used
						//  instead of manually crafting this relabel config.
						{
							SourceLabels: []monitoringv1.LabelName{
								"__meta_kubernetes_pod_annotation_prometheus_io_scrape",
								"__meta_kubernetes_pod_annotation_prometheus_io_port",
							},
							Regex:  `true;(.+)`,
							Action: "keep",
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_annotation_prometheus_io_name"},
							Regex:        `(.+)`,
							Action:       "replace",
							TargetLabel:  "job",
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_annotation_prometheus_io_scheme"},
							Regex:        `(https?)`,
							Action:       "replace",
							TargetLabel:  "__scheme__",
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__address__", "__meta_kubernetes_pod_annotation_prometheus_io_port"},
							Regex:        `([^:]+)(?::\d+)?;(\d+)`,
							Replacement:  ptr.To(`$1:$2`),
							Action:       "replace",
							TargetLabel:  "__address__",
						},
						{
							Action: "labelmap",
							Regex:  `__meta_kubernetes_pod_label_(.+)`,
						},
					},
				}},
			},
		},
	}
}
