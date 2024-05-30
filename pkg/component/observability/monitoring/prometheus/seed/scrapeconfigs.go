// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

// CentralScrapeConfigs returns the central ScrapeConfig resources for the seed prometheus.
func CentralScrapeConfigs() []*monitoringv1alpha1.ScrapeConfig {
	return []*monitoringv1alpha1.ScrapeConfig{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "prometheus",
			},
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				RelabelConfigs: []monitoringv1.RelabelConfig{{
					Action:      "replace",
					Replacement: ptr.To("prometheus"),
					TargetLabel: "job",
				}},
				StaticConfigs: []monitoringv1alpha1.StaticConfig{{
					Targets: []monitoringv1alpha1.Target{"localhost:9090"},
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cadvisor",
			},
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				HonorLabels:     ptr.To(true),
				HonorTimestamps: ptr.To(false),
				MetricsPath:     ptr.To("/federate"),
				Params: map[string][]string{
					"match[]": {
						`{job="cadvisor",namespace=~"extension-(.+)"}`,
						`{job="cadvisor",namespace="garden"}`,
						`{job="cadvisor",namespace=~"istio-(.+)"}`,
						`{job="cadvisor",namespace="kube-system"}`,
					},
				},
				StaticConfigs: []monitoringv1alpha1.StaticConfig{{
					Targets: []monitoringv1alpha1.Target{"prometheus-cache.garden.svc"},
				}},
				RelabelConfigs: []monitoringv1.RelabelConfig{{
					Action:      "replace",
					Replacement: ptr.To("cadvisor"),
					TargetLabel: "job",
				}},
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
					"container_cpu_cfs_periods_total",
					"container_cpu_cfs_throttled_periods_total",
					"container_cpu_cfs_throttled_seconds_total",
					"container_cpu_usage_seconds_total",
					"container_fs_inodes_total",
					"container_fs_limit_bytes",
					"container_fs_usage_bytes",
					"container_last_seen",
					"container_memory_working_set_bytes",
					"container_network_receive_bytes_total",
					"container_network_transmit_bytes_total",
					"container_oom_events_total",
				),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cert-controller-manager",
			},
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				HonorLabels:     ptr.To(true),
				HonorTimestamps: ptr.To(false),
				MetricsPath:     ptr.To("/metrics"),
				StaticConfigs: []monitoringv1alpha1.StaticConfig{{
					Targets: []monitoringv1alpha1.Target{"cert-controller-manager:10258"},
				}},
				RelabelConfigs: []monitoringv1.RelabelConfig{{
					Action:      "replace",
					Replacement: ptr.To("cert-controller-manager"),
					TargetLabel: "job",
				}},
				MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
					Action:      "replace",
					TargetLabel: "__name__",
					Regex:       "promhttp_metric_handler_requests_total",
					Replacement: ptr.To("cert_manager_promhttp_metric_handler_requests_total"),
				}},
			},
		},
	}
}
