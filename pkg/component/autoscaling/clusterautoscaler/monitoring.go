// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clusterautoscaler

import (
	"bytes"
	"strings"
	"text/template"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

const (
	monitoringPrometheusJobName = "cluster-autoscaler"

	monitoringMetricProcessMaxFds  = "process_max_fds"
	monitoringMetricProcessOpenFds = "process_open_fds"

	monitoringMetricClusterAutoscalerClusterSafeToAutoscale           = "cluster_autoscaler_cluster_safe_to_autoscale"
	monitoringMetricClusterAutoscalerNodesCount                       = "cluster_autoscaler_nodes_count"
	monitoringMetricClusterAutoscalerUnschedulablePodsCount           = "cluster_autoscaler_unschedulable_pods_count"
	monitoringMetricClusterAutoscalerNodeGroupsCount                  = "cluster_autoscaler_node_groups_count"
	monitoringMetricClusterAutoscalerMaxNodesCount                    = "cluster_autoscaler_max_nodes_count"
	monitoringMetricClusterAutoscalerClusterCpuCurrentCores           = "cluster_autoscaler_cluster_cpu_current_cores"
	monitoringMetricClusterAutoscalerCpuLimitsCores                   = "cluster_autoscaler_cpu_limits_cores"
	monitoringMetricClusterAutoscalerClusterMemoryCurrentBytes        = "cluster_autoscaler_cluster_memory_current_bytes"
	monitoringMetricClusterAutoscalerMemoryLimitsBytes                = "cluster_autoscaler_memory_limits_bytes"
	monitoringMetricClusterAutoscalerLastActivity                     = "cluster_autoscaler_last_activity"
	monitoringMetricClusterAutoscalerFunctionDurationSeconds          = "cluster_autoscaler_function_duration_seconds"
	monitoringMetricClusterAutoscalerErrorsTotal                      = "cluster_autoscaler_errors_total"
	monitoringMetricClusterAutoscalerScaledUpNodesTotal               = "cluster_autoscaler_scaled_up_nodes_total"
	monitoringMetricClusterAutoscalerScaledDownNodesTotal             = "cluster_autoscaler_scaled_down_nodes_total"
	monitoringMetricClusterAutoscalerScaledUpGpuNodesTotal            = "cluster_autoscaler_scaled_up_gpu_nodes_total"
	monitoringMetricClusterAutoscalerScaledDownGpuNodesTotal          = "cluster_autoscaler_scaled_down_gpu_nodes_total"
	monitoringMetricClusterAutoscalerFailedScaleUpsTotal              = "cluster_autoscaler_failed_scale_ups_total"
	monitoringMetricClusterAutoscalerEvictedPodsTotal                 = "cluster_autoscaler_evicted_pods_total"
	monitoringMetricClusterAutoscalerUnneededNodesCount               = "cluster_autoscaler_unneeded_nodes_count"
	monitoringMetricClusterAutoscalerOldUnregisteredNodesRemovedCount = "cluster_autoscaler_old_unregistered_nodes_removed_count"
	monitoringMetricClusterAutoscalerSkippedScaleEventsCount          = "cluster_autoscaler_skipped_scale_events_count"

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
		monitoringMetricClusterAutoscalerClusterSafeToAutoscale,
		monitoringMetricClusterAutoscalerNodesCount,
		monitoringMetricClusterAutoscalerUnschedulablePodsCount,
		monitoringMetricClusterAutoscalerNodeGroupsCount,
		monitoringMetricClusterAutoscalerMaxNodesCount,
		monitoringMetricClusterAutoscalerClusterCpuCurrentCores,
		monitoringMetricClusterAutoscalerCpuLimitsCores,
		monitoringMetricClusterAutoscalerClusterMemoryCurrentBytes,
		monitoringMetricClusterAutoscalerMemoryLimitsBytes,
		monitoringMetricClusterAutoscalerLastActivity,
		monitoringMetricClusterAutoscalerFunctionDurationSeconds,
		monitoringMetricClusterAutoscalerErrorsTotal,
		monitoringMetricClusterAutoscalerScaledUpNodesTotal,
		monitoringMetricClusterAutoscalerScaledDownNodesTotal,
		monitoringMetricClusterAutoscalerScaledUpGpuNodesTotal,
		monitoringMetricClusterAutoscalerScaledDownGpuNodesTotal,
		monitoringMetricClusterAutoscalerFailedScaleUpsTotal,
		monitoringMetricClusterAutoscalerEvictedPodsTotal,
		monitoringMetricClusterAutoscalerUnneededNodesCount,
		monitoringMetricClusterAutoscalerOldUnregisteredNodesRemovedCount,
		monitoringMetricClusterAutoscalerSkippedScaleEventsCount,
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
