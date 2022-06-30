// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hvpa

import (
	"bytes"
	"text/template"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"

	"github.com/Masterminds/sprig"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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

// ScrapeConfigs returns the scrape configurations for Prometheus.
func (h *hvpa) ScrapeConfigs() ([]string, error) {
	var scrapeConfig bytes.Buffer

	if err := monitoringScrapeConfigTemplate.Execute(&scrapeConfig, map[string]interface{}{
		"relabeledNamespace": h.namespace,
		"allowedMetrics":     monitoringAllowedMetrics,
	}); err != nil {
		return nil, err
	}

	return []string{scrapeConfig.String()}, nil
}

// AlertingRules returns the alerting rules for AlertManager.
func (h *hvpa) AlertingRules() (map[string]string, error) {
	return nil, nil
}
