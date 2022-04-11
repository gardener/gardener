// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubecontrollermanager

import (
	"bytes"
	"strings"
	"text/template"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

const (
	monitoringPrometheusJobName             = "kube-controller-manager"
	monitoringMetricRestClientRequestsTotal = "rest_client_requests_total"
	monitoringMetricProcessMaxFds           = "process_max_fds"
	monitoringMetricProcessOpenFds          = "process_open_fds"

	monitoringAlertingRules = `groups:
- name: kube-controller-manager.rules
  rules:
  - alert: KubeControllerManagerDown
    expr: absent(up{job="` + monitoringPrometheusJobName + `"} == 1)
    for: 15m
    labels:
      service: ` + v1beta1constants.DeploymentNameKubeControllerManager + `
      severity: critical
      type: seed
      visibility: all
    annotations:
      description: Deployments and replication controllers are not making progress.
      summary: Kube Controller Manager is down.`
)

var (
	monitoringAllowedMetrics = []string{
		monitoringMetricRestClientRequestsTotal,
		monitoringMetricProcessMaxFds,
		monitoringMetricProcessOpenFds,
	}

	// TODO remove the hard-coded prometheus paths once the prometheus component is refactored
	monitoringScrapeConfigTmpl = `job_name: ` + monitoringPrometheusJobName + `
scheme: https
tls_config:
  insecure_skip_verify: true
authorization:
  type: Bearer
  credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
honor_labels: false
scrape_timeout: 15s
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: [{{ .namespace }}]
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: ` + serviceName + `;` + portNameMetrics + `
- action: labelmap
  regex: __meta_kubernetes_service_label_(.+)
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
metric_relabel_configs:
- source_labels: [ __name__ ]
  regex: ^(` + strings.Join(monitoringAllowedMetrics, "|") + `)$
  action: keep
`

	monitoringScrapeConfigTemplate *template.Template
)

func init() {
	var err error

	monitoringScrapeConfigTemplate, err = template.New("monitoring-scrape-config").Parse(monitoringScrapeConfigTmpl)
	utilruntime.Must(err)
}

// ScrapeConfigs returns the scrape configurations for Prometheus.
func (k *kubeControllerManager) ScrapeConfigs() ([]string, error) {
	var scrapeConfig bytes.Buffer

	if err := monitoringScrapeConfigTemplate.Execute(&scrapeConfig, map[string]interface{}{"namespace": k.namespace}); err != nil {
		return nil, err
	}

	return []string{scrapeConfig.String()}, nil
}

// AlertingRules returns the alerting rules for AlertManager.
func (k *kubeControllerManager) AlertingRules() (map[string]string, error) {
	return map[string]string{"kube-controller-manager.rules.yaml": monitoringAlertingRules}, nil
}
