// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package hvpa

import (
	"bytes"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
)

const (
	monitoringPrometheusJobName = "hvpa-controller"

	monitoringMetricAggregateAppliedScalingTotal    = "hvpa_aggregate_applied_scaling_total"
	monitoringMetricAggregateBlockedScalingsTotal   = "hvpa_aggregate_blocked_scalings_total"
	monitoringMetricSpecReplicas                    = "hvpa_spec_replicas"
	monitoringMetricStatusReplicas                  = "hvpa_status_replicas"
	monitoringMetricStatusAppliedHpaCurrentReplicas = "hvpa_status_applied_hpa_current_replicas"
	monitoringMetricStatusAppliedHpaDesiredReplicas = "hvpa_status_applied_hpa_desired_replicas"
	monitoringMetricStatusAppliedVpaRecommendation  = "hvpa_status_applied_vpa_recommendation"
	monitoringMetricStatusBlockedHpaCurrentReplicas = "hvpa_status_blocked_hpa_current_replicas"
	monitoringMetricStatusBlockedHpaDesiredReplicas = "hvpa_status_blocked_hpa_desired_replicas"
	monitoringMetricStatusBlockedHpaRecommendation  = "hvpa_status_blocked_vpa_recommendation"
)

var (
	monitoringAllowedMetrics = []string{
		monitoringMetricAggregateAppliedScalingTotal,
		monitoringMetricAggregateBlockedScalingsTotal,
		monitoringMetricSpecReplicas,
		monitoringMetricStatusReplicas,
		monitoringMetricStatusAppliedHpaCurrentReplicas,
		monitoringMetricStatusAppliedHpaDesiredReplicas,
		monitoringMetricStatusAppliedVpaRecommendation,
		monitoringMetricStatusBlockedHpaCurrentReplicas,
		monitoringMetricStatusBlockedHpaDesiredReplicas,
		monitoringMetricStatusBlockedHpaRecommendation,
	}

	monitoringScrapeConfigTmpl = `job_name: ` + monitoringPrometheusJobName + `
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: [ ` + v1beta1constants.GardenNamespace + ` ]
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_endpoint_port_name
  - __meta_kubernetes_namespace
  action: keep
  regex: ` + serviceName + `;` + portNameMetrics + `;` + v1beta1constants.GardenNamespace + `
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^({{ join "|" .allowedMetrics }})$
{{- if .relabeledNamespace }}
- source_labels: [namespace]
  regex: {{ .relabeledNamespace }}
  action: keep
{{- end }}
`
	monitoringScrapeConfigTemplate *template.Template
)

func init() {
	var err error

	monitoringScrapeConfigTemplate, err = template.
		New("monitoring-scrape-config").
		Funcs(sprig.TxtFuncMap()).
		Parse(monitoringScrapeConfigTmpl)
	utilruntime.Must(err)
}

// CentralMonitoringConfiguration returns scrape configs for the central Prometheus.
func CentralMonitoringConfiguration() (component.CentralMonitoringConfig, error) {
	var scrapeConfig bytes.Buffer

	if err := monitoringScrapeConfigTemplate.Execute(&scrapeConfig, map[string]interface{}{
		"allowedMetrics": monitoringAllowedMetrics,
	}); err != nil {
		return component.CentralMonitoringConfig{}, err
	}

	return component.CentralMonitoringConfig{ScrapeConfigs: []string{scrapeConfig.String()}}, nil
}
