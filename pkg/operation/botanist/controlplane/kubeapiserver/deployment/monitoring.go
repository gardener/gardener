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

package deployment

import (
	"bytes"
	"strings"
	"text/template"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

const (
	monitoringPrometheusJobNameAPIServer        = "kube-apiserver"
	monitoringPrometheusJobNameBlackboxExporter = "blackbox-apiserver"

	serviceNameAPIServer = "kube-apiserver"
	serviceNameAuditlog  = "auditlog"

	portNameMetrics = "kube-apiserver"

	monitoringMetricAuditErrorTotal              = "apiserver_audit_error_total"
	monitoringMetricAuditEventTotal              = "apiserver_audit_event_total"
	monitoringMetricCurrentInflightRequests      = "apiserver_current_inflight_requests"
	monitoringMetricCurrentInqueueRequests       = "apiserver_current_inqueue_requests"
	monitoringMetricDroppedRequestsTotal         = "apiserver_dropped_requests_total"
	monitoringMetricRegisteredWatchers           = "apiserver_registered_watchers"
	monitoringMetricRequestCount                 = "apiserver_request_count"
	monitoringMetricRequestDurationSecondsBucket = "apiserver_request_duration_seconds_bucket"
	monitoringMetricRequestTerminationTotal      = "apiserver_request_terminations_total"
	monitoringMetricRequestTotal                 = "apiserver_request_total"
	monitoringMetricEtcdObjectCounts             = "etcd_object_counts"
	monitoringMetricProcessMaxFds                = "process_max_fds"
	monitoringMetricProcessOpenFds               = "process_open_fds"

	monitoringAlertingRules = `groups:
- name: kube-apiserver.rules
  rules:
  - alert: ApiServerNotReachable
    expr: probe_success{job="` + monitoringPrometheusJobNameBlackboxExporter + `"} == 0
    for: 5m
    labels:
      service: ` + v1beta1constants.DeploymentNameKubeAPIServer + `
      severity: blocker
      type: seed
      visibility: all
    annotations:
      description: "API server not reachable via external endpoint: {{ $labels.instance }}."
      summary: API server not reachable (externally).
  - alert: KubeApiserverDown
    expr: absent(up{job="` + monitoringPrometheusJobNameAPIServer + `"} == 1)
    for: 5m
    labels:
      service: ` + v1beta1constants.DeploymentNameKubeAPIServer + `
      severity: blocker
      type: seed
      visibility: operator
    annotations:
      description: All API server replicas are down/unreachable, or all API server could not be found.
      summary: API server unreachable.
  - alert: KubeApiServerTooManyOpenFileDescriptors
    expr: 100 * process_open_fds{job="` + monitoringPrometheusJobNameAPIServer + `"} / process_max_fds > 50
    for: 30m
    labels:
      service: ` + v1beta1constants.DeploymentNameKubeAPIServer + `
      severity: warning
      type: seed
      visibility: owner
    annotations:
      description: 'The API server ({{ $labels.instance }}) is using {{ $value }}% of the available file/socket descriptors.'
      summary: 'The API server has too many open file descriptors'
  - alert: KubeApiServerTooManyOpenFileDescriptors
    expr: 100 * process_open_fds{job="` + monitoringPrometheusJobNameAPIServer + `"} / process_max_fds{job="` + monitoringPrometheusJobNameAPIServer + `"} > 80
    for: 30m
    labels:
      service: ` + v1beta1constants.DeploymentNameKubeAPIServer + `
      severity: critical
      type: seed
      visibility: owner
    annotations:
      description: 'The API server ({{ $labels.instance }}) is using {{ $value }}% of the available file/socket descriptors.'
      summary: 'The API server has too many open file descriptors'
  # Some verbs excluded because they are expected to be long-lasting:
  # WATCHLIST is long-poll, CONNECT is "kubectl exec".
  - alert: KubeApiServerLatency
    expr: histogram_quantile(0.99, sum without (instance,resource) (rate(` + monitoringMetricRequestDurationSecondsBucket + `{subresource!="log",verb!~"CONNECT|WATCHLIST|WATCH|PROXY proxy"}[5m]))) > 3
    for: 30m
    labels:
      service: ` + v1beta1constants.DeploymentNameKubeAPIServer + `
      severity: warning
      type: seed
      visibility: owner
    annotations:
      description: Kube API server latency for verb {{ $labels.verb }} is high. This could be because the shoot workers and the control plane are in different regions. 99th percentile of request latency is greater than 3 seconds.
      summary: Kubernetes API server latency is high
  # TODO replace with better metrics in the future (wyb1)
  - record: shoot:apiserver_watch_duration:quantile
    expr: histogram_quantile(0.2, sum(rate(` + monitoringMetricRequestDurationSecondsBucket + `{verb="WATCH",resource=~"configmaps|deployments|secrets|daemonsets|services|nodes|pods|namespaces|endpoints|statefulsets|clusterroles|roles"}[5m])) by (le,scope,resource))
    labels:
      quantile: "0.2"
  - record: shoot:apiserver_watch_duration:quantile
    expr: histogram_quantile(0.5, sum(rate(` + monitoringMetricRequestDurationSecondsBucket + `{verb="WATCH",resource=~"configmaps|deployments|secrets|daemonsets|services|nodes|pods|namespaces|endpoints|statefulsets|clusterroles|roles"}[5m])) by (le,scope,resource))
    labels:
      quantile: "0.5"
  - record: shoot:apiserver_watch_duration:quantile
    expr: histogram_quantile(0.9, sum(rate(` + monitoringMetricRequestDurationSecondsBucket + `{verb="WATCH",resource=~"configmaps|deployments|secrets|daemonsets|services|nodes|pods|namespaces|endpoints|statefulsets|clusterroles|roles"}[5m])) by (le,scope,resource))
    labels:
      quantile: "0.9"
  - record: shoot:apiserver_watch_duration:quantile
    expr: histogram_quantile(0.2, sum(rate(` + monitoringMetricRequestDurationSecondsBucket + `{verb="WATCH",group=~".+garden.+"}[5m])) by (le,scope,resource))
    labels:
      quantile: "0.2"
  - record: shoot:apiserver_watch_duration:quantile
    expr: histogram_quantile(0.5, sum(rate(` + monitoringMetricRequestDurationSecondsBucket + `{verb="WATCH",group=~".+garden.+"}[5m])) by (le,scope,resource))
    labels:
      quantile: "0.5"
  - record: shoot:apiserver_watch_duration:quantile
    expr: histogram_quantile(0.9, sum(rate(` + monitoringMetricRequestDurationSecondsBucket + `{verb="WATCH",group=~".+garden.+"}[5m])) by (le,scope,resource))
    labels:
      quantile: "0.9"
  ### API auditlog ###
  - alert: KubeApiServerTooManyAuditlogFailures
    expr: sum(rate (` + monitoringMetricAuditErrorTotal + `{plugin="webhook",job="` + monitoringPrometheusJobNameAPIServer + `"}[5m])) / sum(rate(` + monitoringMetricAuditEventTotal + `{job="` + monitoringPrometheusJobNameAPIServer + `"}[5m])) > bool 0.02 == 1
    for: 15m
    labels:
      service: ` + serviceNameAuditlog + `
      severity: warning
      type: seed
      visibility: operator
    annotations:
      description: 'The API servers cumulative failure rate in logging audit events is greater than 2%.'
      summary: 'The kubernetes API server has too many failed attempts to log audit events'
  ### API latency ###
  - record: apiserver_latency_seconds:quantile
    expr: histogram_quantile(0.99, rate(` + monitoringMetricRequestDurationSecondsBucket + `[5m]))
    labels:
      quantile: "0.99"
  - record: apiserver_latency:quantile
    expr: histogram_quantile(0.9, rate(` + monitoringMetricRequestDurationSecondsBucket + `[5m]))
    labels:
      quantile: "0.9"
  - record: apiserver_latency_seconds:quantile
    expr: histogram_quantile(0.5, rate(` + monitoringMetricRequestDurationSecondsBucket + `[5m]))
    labels:
      quantile: "0.5"

  - record: shoot:kube_apiserver:sum_by_pod
    expr: sum(up{job="` + monitoringPrometheusJobNameAPIServer + `"}) by (pod)`
)

var (
	monitoringAllowedMetrics = []string{
		monitoringMetricAuditErrorTotal,
		monitoringMetricAuditEventTotal,
		monitoringMetricCurrentInflightRequests,
		monitoringMetricCurrentInqueueRequests,
		monitoringMetricDroppedRequestsTotal,
		monitoringMetricRegisteredWatchers,
		monitoringMetricRequestCount,
		monitoringMetricRequestDurationSecondsBucket,
		monitoringMetricRequestTerminationTotal,
		monitoringMetricRequestTotal,
		monitoringMetricEtcdObjectCounts,
		monitoringMetricProcessMaxFds,
		monitoringMetricProcessOpenFds,
	}

	// 	insecure TLS config is needed because the api server's certificates are only valid for a domain
	// 	and not for a specific pod IP
	monitoringScrapeConfigTmpl = `job_name: ` + monitoringPrometheusJobNameAPIServer + `
scheme: https
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: [{{ .namespace }}]
tls_config:
  insecure_skip_verify: true
  cert_file: /etc/prometheus/seed/prometheus.crt
  key_file: /etc/prometheus/seed/prometheus.key
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: ` + serviceNameAPIServer + `;` + portNameMetrics + `
- action: labelmap
  regex: __meta_kubernetes_service_label_(.+)
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
metric_relabel_configs:
- source_labels: [ __name__ ]
  regex: ^(` + strings.Join(monitoringAllowedMetrics, "|") + `)$
  action: keep
  {{- if .k8sSmaller114 }}
- source_labels: [ __name__ ]
  regex: ^apiserver_request_count$
  action: replace
  replacement: apiserver_request_total
  target_label: __name__
  {{- end }}
`

	monitoringScrapeConfigTemplate *template.Template
)

func init() {
	var err error

	monitoringScrapeConfigTemplate, err = template.New("monitoring-scrape-config").Parse(monitoringScrapeConfigTmpl)
	utilruntime.Must(err)
}

// ScrapeConfigs returns the scrape configurations for Prometheus.
func (k *kubeAPIServer) ScrapeConfigs() ([]string, error) {
	var scrapeConfig bytes.Buffer
	if err := monitoringScrapeConfigTemplate.Execute(&scrapeConfig, map[string]interface{}{
		"k8sSmaller114": versionConstraintK8sSmaller114.Check(k.shootKubernetesVersion),
		"namespace":     k.seedNamespace,
	}); err != nil {
		return nil, err
	}

	return []string{scrapeConfig.String()}, nil
}

// AlertConfig returns the alerting configuration for AlertManager.
func (k *kubeAPIServer) AlertingRules() (map[string]string, error) {
	return map[string]string{"kube-apiserver.rules.yaml": monitoringAlertingRules}, nil
}
