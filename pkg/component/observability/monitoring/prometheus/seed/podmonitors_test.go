// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/seed"
)

var _ = Describe("PodMonitors", func() {
	Describe("#CentralPodMonitors", func() {
		It("should return the expected objects", func() {
			Expect(seed.CentralPodMonitors()).To(HaveExactElements(
				&monitoringv1.PodMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name: "extensions",
					},
					Spec: monitoringv1.PodMonitorSpec{
						NamespaceSelector: monitoringv1.NamespaceSelector{Any: true},
						PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{
							RelabelConfigs: []monitoringv1.RelabelConfig{
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
				&monitoringv1.PodMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name: "garden",
					},
					Spec: monitoringv1.PodMonitorSpec{
						PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{
							Scheme:    "https",
							TLSConfig: &monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)},
							RelabelConfigs: []monitoringv1.RelabelConfig{
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
			))
		})
	})
})
