// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package kubeproxy

import (
	"strconv"
	"strings"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubeapiserver/constants"
)

const (
	monitoringPrometheusJobName = "kube-proxy"

	monitoringMetricNetworkProgrammingDurationSecondsBucket = "kubeproxy_network_programming_duration_seconds_bucket"
	monitoringMetricNetworkProgrammingDurationSecondsCount  = "kubeproxy_network_programming_duration_seconds_count"
	monitoringMetricNetworkProgrammingDurationSecondsSum    = "kubeproxy_network_programming_duration_seconds_sum"
	monitoringMetricSyncProxyRulesDurationSecondsBucket     = "kubeproxy_sync_proxy_rules_duration_seconds_bucket"
	monitoringMetricSyncProxyRulesDurationSecondsCount      = "kubeproxy_sync_proxy_rules_duration_seconds_count"
	monitoringMetricSyncProxyRulesDurationSecondsSum        = "kubeproxy_sync_proxy_rules_duration_seconds_sum"

	monitoringAlertingRules = `groups:
- name: kube-proxy.rules
  rules:
  - record: kubeproxy_network_latency:quantile
    expr: histogram_quantile(0.99, sum(rate(` + monitoringMetricNetworkProgrammingDurationSecondsBucket + `[10m])) by (le))
    labels:
      quantile: "0.99"
  - record: kubeproxy_network_latency:quantile
    expr: histogram_quantile(0.9, sum(rate(` + monitoringMetricNetworkProgrammingDurationSecondsBucket + `[10m])) by (le))
    labels:
      quantile: "0.9"
  - record: kubeproxy_network_latency:quantile
    expr: histogram_quantile(0.5, sum(rate(` + monitoringMetricNetworkProgrammingDurationSecondsBucket + `[10m])) by (le))
    labels:
      quantile: "0.5"
  - record: kubeproxy_sync_proxy:quantile
    expr: histogram_quantile(0.99, sum(rate(` + monitoringMetricSyncProxyRulesDurationSecondsBucket + `[10m])) by (le))
    labels:
      quantile: "0.99"
  - record: kubeproxy_sync_proxy:quantile
    expr: histogram_quantile(0.9, sum(rate(` + monitoringMetricSyncProxyRulesDurationSecondsBucket + `[10m])) by (le))
    labels:
      quantile: "0.9"
  - record: kubeproxy_sync_proxy:quantile
    expr: histogram_quantile(0.5, sum(rate(` + monitoringMetricSyncProxyRulesDurationSecondsBucket + `[10m])) by (le))
    labels:
      quantile: "0.5"
`
)

var (
	monitoringAllowedMetrics = []string{
		monitoringMetricNetworkProgrammingDurationSecondsBucket,
		monitoringMetricNetworkProgrammingDurationSecondsCount,
		monitoringMetricNetworkProgrammingDurationSecondsSum,
		monitoringMetricSyncProxyRulesDurationSecondsBucket,
		monitoringMetricSyncProxyRulesDurationSecondsCount,
		monitoringMetricSyncProxyRulesDurationSecondsSum,
	}

	// TODO: Replace below hard-coded paths to Prometheus certificates once its deployment has been refactored.
	monitoringScrapeConfig = `job_name: ` + monitoringPrometheusJobName + `
honor_labels: false
scheme: https
tls_config:
  ca_file: /etc/prometheus/seed/ca.crt
authorization:
  type: Bearer
  credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
follow_redirects: false
kubernetes_sd_configs:
- role: endpoints
  api_server: https://` + v1beta1constants.DeploymentNameKubeAPIServer + `:` + strconv.Itoa(kubeapiserverconstants.Port) + `
  namespaces:
    names: [ kube-system ]
  tls_config:
    ca_file: /etc/prometheus/seed/ca.crt
  authorization:
    type: Bearer
    credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
relabel_configs:
- source_labels:
  - __meta_kubernetes_endpoints_name
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: ` + serviceName + `;` + portNameMetrics + `
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
- source_labels: [ __meta_kubernetes_pod_node_name ]
  target_label: node
- target_label: __address__
  replacement: ` + v1beta1constants.DeploymentNameKubeAPIServer + `:` + strconv.Itoa(kubeapiserverconstants.Port) + `
- source_labels: [__meta_kubernetes_pod_name, __meta_kubernetes_pod_container_port_number]
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
func (k *kubeProxy) ScrapeConfigs() ([]string, error) {
	return []string{monitoringScrapeConfig}, nil
}

// AlertingRules returns the alerting rules for AlertManager.
func (k *kubeProxy) AlertingRules() (map[string]string, error) {
	return map[string]string{"kube-proxy.rules.yaml": monitoringAlertingRules}, nil
}
