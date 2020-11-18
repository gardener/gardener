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

package clusterautoscaler

import (
	"bytes"
	"strings"
	"text/template"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

const (
	monitoringPrometheusJobName = "cluster-autoscaler"

	monitoringMetricProcessMaxFds  = "process_max_fds"
	monitoringMetricProcessOpenFds = "process_open_fds"

	monitoringAlertingRules = `groups:
- name: cluster-autoscaler.rules
  rules:
  - alert: ClusterAutoscalerDown
    expr: absent(up{job="` + monitoringPrometheusJobName + `"} == 1)
    for: 7m
    labels:
      service: ` + v1beta1constants.DeploymentNameClusterAutoscaler + `
      severity: critical
      type: seed
    annotations:
      description: There is no running cluster autoscaler. Shoot's Nodes wont be scaled dynamically, based on the load.
      summary: Cluster autoscaler is down
`
)

var (
	monitoringAllowedMetrics = []string{
		monitoringMetricProcessMaxFds,
		monitoringMetricProcessOpenFds,
	}

	monitoringScrapeConfigTmpl = `job_name: ` + monitoringPrometheusJobName + `
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
  regex: ` + ServiceName + `;` + portNameMetrics + `
- action: labelmap
  regex: __meta_kubernetes_service_label_(.+)
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(` + strings.Join(monitoringAllowedMetrics, "|") + `)$
`

	monitoringScrapeConfigTemplate *template.Template
)

func init() {
	var err error

	monitoringScrapeConfigTemplate, err = template.New("monitoring-scrape-config").Parse(monitoringScrapeConfigTmpl)
	utilruntime.Must(err)
}

// ScrapeConfigs returns the scrape configurations for Prometheus.
func (c *clusterAutoscaler) ScrapeConfigs() ([]string, error) {
	var scrapeConfig bytes.Buffer

	if err := monitoringScrapeConfigTemplate.Execute(&scrapeConfig, map[string]interface{}{"namespace": c.namespace}); err != nil {
		return nil, err
	}

	return []string{scrapeConfig.String()}, nil
}

// AlertingRules returns the alerting rules for AlertManager.
func (c *clusterAutoscaler) AlertingRules() (map[string]string, error) {
	return map[string]string{"cluster-autoscaler.rules.yaml": monitoringAlertingRules}, nil
}
