// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package longterm

import (
	_ "embed"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
)

// CentralScrapeConfigs returns the central ScrapeConfig resources for the garden prometheus.
func CentralScrapeConfigs() []*monitoringv1alpha1.ScrapeConfig {
	return []*monitoringv1alpha1.ScrapeConfig{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "prometheus"},
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				StaticConfigs: []monitoringv1alpha1.StaticConfig{{
					Targets: []monitoringv1alpha1.Target{"localhost:9090"},
				}},
				RelabelConfigs: []monitoringv1.RelabelConfig{{
					Action:      "replace",
					Replacement: ptr.To("prometheus"),
					TargetLabel: "job",
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "cortex-frontend"},
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				StaticConfigs: []monitoringv1alpha1.StaticConfig{{
					Targets: []monitoringv1alpha1.Target{"localhost:9091"},
				}},
				RelabelConfigs: []monitoringv1.RelabelConfig{{
					Action:      "replace",
					Replacement: ptr.To("cortex-frontend"),
					TargetLabel: "job",
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "prometheus-" + garden.Label},
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				HonorLabels:     ptr.To(true),
				HonorTimestamps: ptr.To(true),
				MetricsPath:     ptr.To("/federate"),
				Params: map[string][]string{
					"match[]": {
						`{__name__="garden_shoot_info"}`,
						`{__name__="garden_seed_info"}`,
						`{__name__=~"garden_shoot_info:timestamp:this_month"}`,
						`{__name__=~"metering:(cpu_requests|memory_requests|network|persistent_volume_claims|disk_usage_seconds|memory_usage_seconds).*:this_month"}`,
						`{__name__="garden_shoot_node_info"}`,
						`{__name__="garden_shoot_condition", condition=~"(APIServerAvailable|SystemComponentsHealthy)"}`,
						`{__name__="garden_seed_condition", condition=~"(SeedSystemComponentsHealthy|GardenletReady)"}`,
						`{__name__="garden_seed_usage"}`,
						`{__name__="garden_seed_capacity"}`,
						`{__name__="etcdbr_snapshot_duration_seconds_count"}`,
						`{__name__="apiserver_request_total", job="virtual-garden-kube-apiserver"}`,
					},
				},
				KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
					Role:       monitoringv1alpha1.KubernetesRoleService,
					Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{v1beta1constants.GardenNamespace}},
				}},
				RelabelConfigs: []monitoringv1.RelabelConfig{
					{
						SourceLabels: []monitoringv1.LabelName{
							"__meta_kubernetes_service_name",
							"__meta_kubernetes_service_port_name",
						},
						Regex:  "prometheus-garden;" + prometheus.ServicePortName,
						Action: "keep",
					},
					{
						Action:      "replace",
						Replacement: ptr.To("prometheus-" + garden.Label),
						TargetLabel: "job",
					}},
			},
		},
	}
}
