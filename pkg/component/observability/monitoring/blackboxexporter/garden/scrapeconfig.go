// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	defaultKeepMetrics := []string{
		"probe_success",
		"probe_http_status_code",
		"probe_http_duration_seconds",
	}
	additionalKeepTLSMetrics := []string{
		"probe_ssl_earliest_cert_expiry",
	}
	genScrapeConfig := func(name, module string, targets []monitoringv1alpha1.Target, metrics []string) *monitoringv1alpha1.ScrapeConfig {
		return &monitoringv1alpha1.ScrapeConfig{
			ObjectMeta: monitoringutils.ConfigObjectMeta("blackbox-"+name, namespace, gardenprometheus.Label),
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				Params:      map[string][]string{"module": {module}},
				MetricsPath: ptr.To("/probe"),
				StaticConfigs: []monitoringv1alpha1.StaticConfig{{
					Targets: targets,
					Labels:  map[monitoringv1.LabelName]string{"purpose": "availability"},
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
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(metrics...),
			},
		}
	}
	defaultScrapeConfig := func(name, module string, targets []monitoringv1alpha1.Target) *monitoringv1alpha1.ScrapeConfig {
		return genScrapeConfig(name, module, targets, defaultKeepMetrics)
	}
	tlsScrapeConfig := func(name, module string, targets []monitoringv1alpha1.Target) *monitoringv1alpha1.ScrapeConfig {
		return genScrapeConfig(name, module, targets, append(defaultKeepMetrics, additionalKeepTLSMetrics...))
	}

	var (
		gardenerAPIServerScrapeConfig       = tlsScrapeConfig("gardener-apiserver", httpGardenerAPIServerModuleName, []monitoringv1alpha1.Target{"https://gardener-apiserver.garden.svc/healthz"})
		kubeAPIServerScrapeConfig           = tlsScrapeConfig("apiserver", httpKubeAPIServerModuleName, kubeAPIServerTargets)
		gardenerDashboardScrapeConfig       = tlsScrapeConfig("dashboard", httpGardenerDashboardModuleName, []monitoringv1alpha1.Target{gardenerDashboardTarget})
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
