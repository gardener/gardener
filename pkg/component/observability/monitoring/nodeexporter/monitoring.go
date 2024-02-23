// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nodeexporter

import (
	"strconv"
	"strings"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubeapiserver/constants"
)

const (
	monitoringPrometheusJobName = "node-exporter"

	monitoringMetricNodeBootTimeSeconds                = "node_boot_time_seconds"
	monitoringMetricNodeCpuSecondsTotal                = "node_cpu_seconds_total"
	monitoringMetricNodeDiskReadBytesTotal             = "node_disk_read_bytes_total"
	monitoringMetricNodeDiskWrittenBytesTotal          = "node_disk_written_bytes_total"
	monitoringMetricNodeDiskIoTimeWeightedSecondsTotal = "node_disk_io_time_weighted_seconds_total"
	monitoringMetricNodeDiskIoTimeSecondsTotal         = "node_disk_io_time_seconds_total"
	monitoringMetricNodeDiskWriteTimeSecondsTotal      = "node_disk_write_time_seconds_total"
	monitoringMetricNodeDiskWritesCompletedTotal       = "node_disk_writes_completed_total"
	monitoringMetricNodeDiskReadTimeSecondsTotal       = "node_disk_read_time_seconds_total"
	monitoringMetricNodeDiskReadsCompletedTotal        = "node_disk_reads_completed_total"
	monitoringMetricNodeFilesystemAvailBytes           = "node_filesystem_avail_bytes"
	monitoringMetricNodeFilesystemFiles                = "node_filesystem_files"
	monitoringMetricNodeFilesystemFilesFree            = "node_filesystem_files_free"
	monitoringMetricNodeFilesystemFreeBytes            = "node_filesystem_free_bytes"
	monitoringMetricNodeFilesystemReadonly             = "node_filesystem_readonly"
	monitoringMetricNodeFilesystemSizeBytes            = "node_filesystem_size_bytes"
	monitoringMetricNodeLoad1                          = "node_load1"
	monitoringMetricNodeLoad15                         = "node_load15"
	monitoringMetricNodeLoad5                          = "node_load5"
	monitoringMetricNodeMemory                         = "node_memory_.+"
	monitoringMetricNodeNfConntrack                    = "node_nf_conntrack_.+"
	monitoringMetricNodeScrapeCollectorDurationSeconds = "node_scrape_collector_duration_seconds"
	monitoringMetricNodeScrapeCollectorSuccess         = "node_scrape_collector_success"
	monitoringMetricProcessMaxFds                      = "process_max_fds"
	monitoringMetricProcessOpenFds                     = "process_open_fds"

	monitoringAlertingRules = `groups:
- name: node-exporter.rules
  rules:
  - alert: NodeExporterDown
    expr: absent(up{job="` + monitoringPrometheusJobName + `"} == 1)
    for: 1h
    labels:
      service: ` + serviceName + `
      severity: warning
      type: shoot
      visibility: owner
    annotations:
      summary: NodeExporter down or unreachable
      description: The NodeExporter has been down or unreachable from Prometheus for more than 1 hour.

  - alert: K8SNodeOutOfDisk
    expr: kube_node_status_condition{condition="OutOfDisk", status="true"} == 1
    for: 1h
    labels:
      service: ` + serviceName + `
      severity: critical
      type: shoot
      visibility: owner
    annotations:
      summary: Node ran out of disk space.
      description: Node {{ $labels.node }} has run out of disk space.

  - alert: K8SNodeMemoryPressure
    expr: kube_node_status_condition{condition="MemoryPressure", status="true"} == 1
    for: 1h
    labels:
      service: ` + serviceName + `
      severity: warning
      type: shoot
      visibility: owner
    annotations:
      summary: Node is under memory pressure.
      description: Node {{ $labels.node }} is under memory pressure.

  - alert: K8SNodeDiskPressure
    expr: kube_node_status_condition{condition="DiskPressure", status="true"} == 1
    for: 1h
    labels:
      service: ` + serviceName + `
      severity: warning
      type: shoot
      visibility: owner
    annotations:
      summary: Node is under disk pressure.
      description: Node {{ $labels.node }} is under disk pressure

  - record: instance:conntrack_entries_usage:percent
    expr: (node_nf_conntrack_entries / node_nf_conntrack_entries_limit) * 100

  # alert if the root filesystem is full
  - alert: VMRootfsFull
    expr: node_filesystem_free{mountpoint="/"} < 1024
    for: 1h
    labels:
      service: ` + serviceName + `
      severity: critical
      type: shoot
      visibility: owner
    annotations:
      description: Root filesystem device on instance {{ $labels.instance }} is almost full.
      summary: Node's root filesystem is almost full

  - alert: VMConntrackTableFull
    for: 1h
    expr: instance:conntrack_entries_usage:percent > 90
    labels:
      service: ` + serviceName + `
      severity: critical
      type: shoot
      visibility: owner
    annotations:
      description: The nf_conntrack table is {{ $value }}% full.
      summary: Number of tracked connections is near the limit

  - record: shoot:kube_node_info:count
    expr: count(kube_node_info{type="shoot"})

  # This recording rule creates a series for nodes with less than 5% free inodes on a not read only mount point.
  # The series exists only if there are less than 5% free inodes,
  # to keep the cardinality of these federated metrics manageable.
  # Otherwise we would get a series for each node in each shoot in the federating Prometheus.
  - record: shoot:node_filesystem_files_free:percent
    expr: |
      sum by (node, mountpoint)
        (node_filesystem_files_free / node_filesystem_files * 100 < 5
         and node_filesystem_readonly == 0)
`
)

var (
	monitoringAllowedMetrics = []string{
		monitoringMetricNodeBootTimeSeconds,
		monitoringMetricNodeCpuSecondsTotal,
		monitoringMetricNodeDiskReadBytesTotal,
		monitoringMetricNodeDiskWrittenBytesTotal,
		monitoringMetricNodeDiskIoTimeWeightedSecondsTotal,
		monitoringMetricNodeDiskIoTimeSecondsTotal,
		monitoringMetricNodeDiskWriteTimeSecondsTotal,
		monitoringMetricNodeDiskWritesCompletedTotal,
		monitoringMetricNodeDiskReadTimeSecondsTotal,
		monitoringMetricNodeDiskReadsCompletedTotal,
		monitoringMetricNodeFilesystemAvailBytes,
		monitoringMetricNodeFilesystemFiles,
		monitoringMetricNodeFilesystemFilesFree,
		monitoringMetricNodeFilesystemFreeBytes,
		monitoringMetricNodeFilesystemReadonly,
		monitoringMetricNodeFilesystemSizeBytes,
		monitoringMetricNodeLoad1,
		monitoringMetricNodeLoad15,
		monitoringMetricNodeLoad5,
		monitoringMetricNodeMemory,
		monitoringMetricNodeNfConntrack,
		monitoringMetricNodeScrapeCollectorDurationSeconds,
		monitoringMetricNodeScrapeCollectorSuccess,
		monitoringMetricProcessMaxFds,
		monitoringMetricProcessOpenFds,
	}

	monitoringScrapeConfig = `job_name: ` + monitoringPrometheusJobName + `
honor_labels: false
scrape_timeout: 30s
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
  tls_config:
    ca_file: /etc/prometheus/seed/ca.crt
  authorization:
    type: Bearer
    credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
  namespaces:
    names: [ kube-system ]
relabel_configs:
- target_label: type
  replacement: shoot
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: ` + serviceName + `;` + portNameMetrics + `
# common metrics
- action: labelmap
  regex: __meta_kubernetes_service_label_(.+)
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

// ScrapeConfigs returns the scrape configurations for node-exporter.
func (n *nodeExporter) ScrapeConfigs() ([]string, error) {
	return []string{monitoringScrapeConfig}, nil
}

// AlertingRules returns the alerting rules for AlertManager.
func (n *nodeExporter) AlertingRules() (map[string]string, error) {
	return map[string]string{"node-exporter.rules.yaml": monitoringAlertingRules}, nil
}
