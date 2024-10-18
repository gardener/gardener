// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package aggregate

import (
	_ "embed"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus"
)

// CentralScrapeConfigs returns the central ScrapeConfig resources for the aggregate prometheus.
func CentralScrapeConfigs() []*monitoringv1alpha1.ScrapeConfig {
	return []*monitoringv1alpha1.ScrapeConfig{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "prometheus"},
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				HonorTimestamps: ptr.To(false),
				MetricsPath:     ptr.To("/federate"),
				Params: map[string][]string{
					"match[]": {
						`{__name__=~"metering:.+", __name__!~"metering:.+(over_time|_seconds|:this_month)"}`,
						`{__name__=~"seed:(.+):(.+)"}`,
						`{job="kube-state-metrics",namespace=~"garden|extension-.+"}`,
						`{job="kube-state-metrics",namespace=""}`,
						`{job="cadvisor",namespace=~"garden|extension-.+"}`,
						`{job="etcd-druid",namespace="garden"}`,
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
						Regex:  "prometheus-cache;" + prometheus.ServicePortName,
						Action: "keep",
					},
					{
						Action:      "replace",
						Replacement: ptr.To("prometheus"),
						TargetLabel: "job",
					},
				},
			},
		},
	}
}
