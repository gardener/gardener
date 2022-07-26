// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package coredns

import (
	"strconv"
	"strings"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
)

const (
	monitoringPrometheusJobName = "coredns"

	monitoringMetricBuildInfo                                     = "coredns_build_info"
	monitoringMetricCacheEntries                                  = "coredns_cache_entries"
	monitoringMetricCacheHitsTotal                                = "coredns_cache_hits_total"
	monitoringMetricCacheMissesTotal                              = "coredns_cache_misses_total"
	monitoringMetricDnsRequestDurationSecondsCount                = "coredns_dns_request_duration_seconds_count"
	monitoringMetricDnsRequestDurationSecondsBucket               = "coredns_dns_request_duration_seconds_bucket"
	monitoringMetricDnsRequestsTotal                              = "coredns_dns_requests_total"
	monitoringMetricDnsResponsesTotal                             = "coredns_dns_responses_total"
	monitoringMetricForwardRequestsTotal                          = "coredns_forward_requests_total"
	monitoringMetricForwardResponsesTotal                         = "coredns_forward_responses_total"
	monitoringMetricKubernetesDnsProgrammingDurationSecondsBucket = "coredns_kubernetes_dns_programming_duration_seconds_bucket"
	monitoringMetricKubernetesDnsProgrammingDurationSecondsCount  = "coredns_kubernetes_dns_programming_duration_seconds_count"
	monitoringMetricKubernetesDnsProgrammingDurationSecondsSum    = "coredns_kubernetes_dns_programming_duration_seconds_sum"
	monitoringMetricProcessMaxFds                                 = "process_max_fds"
	monitoringMetricProcessOpenFds                                = "process_open_fds"

	monitoringAlertingRules = `groups:
- name: coredns.rules
  rules:
  - alert: CoreDNSDown
    expr: absent(up{job="` + monitoringPrometheusJobName + `"} == 1)
    for: 20m
    labels:
      service: ` + serviceName + `
      severity: critical
      type: shoot
      visibility: all
    annotations:
      description: CoreDNS could not be found. Cluster DNS resolution will not work.
      summary: CoreDNS is down
`
)

var (
	monitoringAllowedMetrics = []string{
		monitoringMetricBuildInfo,
		monitoringMetricCacheEntries,
		monitoringMetricCacheHitsTotal,
		monitoringMetricCacheMissesTotal,
		monitoringMetricDnsRequestDurationSecondsCount,
		monitoringMetricDnsRequestDurationSecondsBucket,
		monitoringMetricDnsRequestsTotal,
		monitoringMetricDnsResponsesTotal,
		monitoringMetricForwardRequestsTotal,
		monitoringMetricForwardResponsesTotal,
		monitoringMetricKubernetesDnsProgrammingDurationSecondsBucket,
		monitoringMetricKubernetesDnsProgrammingDurationSecondsCount,
		monitoringMetricKubernetesDnsProgrammingDurationSecondsSum,
		monitoringMetricProcessMaxFds,
		monitoringMetricProcessOpenFds,
	}

	// TODO: Replace below hard-coded paths to Prometheus certificates once its deployment has been refactored.
	monitoringScrapeConfig = `job_name: ` + monitoringPrometheusJobName + `
scheme: https
tls_config:
  ca_file: /etc/prometheus/seed/ca.crt
authorization:
  type: Bearer
  credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
honor_labels: false
kubernetes_sd_configs:
- role: endpoints
  api_server: https://` + v1beta1constants.DeploymentNameKubeAPIServer + `:` + strconv.Itoa(kubeapiserver.Port) + `
  tls_config:
    ca_file: /etc/prometheus/seed/ca.crt
  authorization:
    type: Bearer
    credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
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
- target_label: __address__
  replacement: ` + v1beta1constants.DeploymentNameKubeAPIServer + `:` + strconv.Itoa(kubeapiserver.Port) + `
- source_labels: [__meta_kubernetes_pod_name,__meta_kubernetes_pod_container_port_number]
  regex: (.+);(.+)
  target_label: __metrics_path__
  replacement: /api/v1/namespaces/kube-system/pods/${1}:${2}/proxy/metrics
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(` + strings.Join(monitoringAllowedMetrics, "|") + `)$
`
)

// ScrapeConfigs returns the scrape configurations for Prometheus.
func (c *coreDNS) ScrapeConfigs() ([]string, error) {
	return []string{monitoringScrapeConfig}, nil
}

// AlertingRules returns the alerting rules for AlertManager.
func (c *coreDNS) AlertingRules() (map[string]string, error) {
	return map[string]string{"coredns.rules.yaml": monitoringAlertingRules}, nil
}
