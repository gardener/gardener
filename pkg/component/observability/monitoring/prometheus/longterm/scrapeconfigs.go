// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package longterm

import (
	_ "embed"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

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
						`{__name__="apiserver_request_total"}`,
					},
				},
				StaticConfigs: []monitoringv1alpha1.StaticConfig{{Targets: []monitoringv1alpha1.Target{"prometheus-" + garden.Label}}},
				RelabelConfigs: []monitoringv1.RelabelConfig{{
					Action:      "replace",
					Replacement: ptr.To("prometheus-" + garden.Label),
					TargetLabel: "job",
				}},
			},
		},
	}
}
