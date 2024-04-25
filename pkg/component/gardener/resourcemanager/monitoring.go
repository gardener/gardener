// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcemanager

import (
	"bytes"
	"text/template"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	resourcemanagerconstants "github.com/gardener/gardener/pkg/component/gardener/resourcemanager/constants"
)

const (
	monitoringPrometheusJobName = "gardener-resource-manager"
)

var (
	monitoringScrapeConfigTmpl = `job_name: {{ .namePrefix }}` + monitoringPrometheusJobName + `
honor_labels: false
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: [{{ .namespace }}]
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: {{ .namePrefix }}` + resourcemanagerconstants.ServiceName + `;` + metricsPortName + `
- action: labelmap
  regex: __meta_kubernetes_service_label_(.+)
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
- source_labels: [ __meta_kubernetes_namespace ]
  target_label: namespace
`

	monitoringScrapeConfigTemplate *template.Template
)

func init() {
	var err error

	monitoringScrapeConfigTemplate, err = template.New("monitoring-scrape-config").Parse(monitoringScrapeConfigTmpl)
	utilruntime.Must(err)
}

// ScrapeConfigs returns the scrape configurations for Prometheus.
func (r *resourceManager) ScrapeConfigs() ([]string, error) {
	var scrapeConfig bytes.Buffer

	if err := monitoringScrapeConfigTemplate.Execute(&scrapeConfig, map[string]interface{}{"namespace": r.namespace, "namePrefix": r.values.NamePrefix}); err != nil {
		return nil, err
	}

	return []string{scrapeConfig.String()}, nil
}

// AlertingRules returns the alerting rules for AlertManager.
func (r *resourceManager) AlertingRules() (map[string]string, error) {
	return nil, nil
}
