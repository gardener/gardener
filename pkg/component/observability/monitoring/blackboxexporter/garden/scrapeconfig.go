// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"k8s.io/utils/ptr"

	gardenprometheus "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

// ScrapeConfig returns the scrape configs related to the blackbox-exporter for the garden use-case.
func ScrapeConfig(namespace string, kubeAPIServerTargets []monitoringv1alpha1.Target, gardenerDashboardTarget monitoringv1alpha1.Target) []*monitoringv1alpha1.ScrapeConfig {
	defaultScrapeConfig := func(name, module string, targets []monitoringv1alpha1.Target) *monitoringv1alpha1.ScrapeConfig {
		return &monitoringv1alpha1.ScrapeConfig{
			ObjectMeta: monitoringutils.ConfigObjectMeta("blackbox-"+name, namespace, gardenprometheus.Label),
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				Params:      map[string][]string{"module": {module}},
				MetricsPath: ptr.To("/probe"),
				StaticConfigs: []monitoringv1alpha1.StaticConfig{{
					Targets: targets,
					Labels:  map[string]string{"purpose": "availability"},
				}},
				RelabelConfigs: []monitoringv1.RelabelConfig{
					{
						SourceLabels: []monitoringv1.LabelName{"__address__"},
						Separator:    ptr.To(";"),
						Regex:        `(.*)`,
						TargetLabel:  "__param_target",
						Replacement:  ptr.To(`$1`),
						Action:       "replace",
					},
					{
						SourceLabels: []monitoringv1.LabelName{"__param_target"},
						Separator:    ptr.To(";"),
						Regex:        `(.*)`,
						TargetLabel:  "instance",
						Replacement:  ptr.To(`$1`),
						Action:       "replace",
					},
					{
						Separator:   ptr.To(";"),
						Regex:       `(.*)`,
						TargetLabel: "__address__",
						Replacement: ptr.To("blackbox-exporter:9115"),
						Action:      "replace",
					},
					{
						Action:      "replace",
						Replacement: ptr.To("blackbox-" + name),
						TargetLabel: "job",
					},
				},
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
					"probe_success",
					"probe_http_status_code",
					"probe_http_duration_seconds",
				),
			},
		}
	}

	var (
		gardenerAPIServerScrapeConfig       = defaultScrapeConfig("gardener-apiserver", httpGardenerAPIServerModuleName, []monitoringv1alpha1.Target{"https://gardener-apiserver.garden.svc/healthz"})
		kubeAPIServerScrapeConfig           = defaultScrapeConfig("apiserver", httpKubeAPIServerModuleName, kubeAPIServerTargets)
		gardenerDashboardScrapeConfig       = defaultScrapeConfig("dashboard", httpGardenerDashboardModuleName, []monitoringv1alpha1.Target{gardenerDashboardTarget})
		gardenerDiscoveryServerScrapeConfig = defaultScrapeConfig("discovery-server", httpGardenerDiscoveryServerModuleName, []monitoringv1alpha1.Target{"http://gardener-discovery-server.garden.svc.cluster.local:8081/healthz"})
	)

	kubeAPIServerScrapeConfig.Spec.RelabelConfigs = append([]monitoringv1.RelabelConfig{{
		SourceLabels: []monitoringv1.LabelName{"__address__"},
		Separator:    ptr.To(";"),
		Regex:        `https://api\..*`,
		TargetLabel:  "__param_module",
		Replacement:  ptr.To(httpKubeAPIServerRootCAsModuleName),
		Action:       "replace",
	}}, kubeAPIServerScrapeConfig.Spec.RelabelConfigs...)

	return []*monitoringv1alpha1.ScrapeConfig{gardenerAPIServerScrapeConfig, kubeAPIServerScrapeConfig, gardenerDashboardScrapeConfig, gardenerDiscoveryServerScrapeConfig}
}
