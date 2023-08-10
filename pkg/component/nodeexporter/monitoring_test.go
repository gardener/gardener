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

package nodeexporter_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"

	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/nodeexporter"
	"github.com/gardener/gardener/pkg/component/test"
)

var _ = Describe("Monitoring", func() {
	var component component.MonitoringComponent

	BeforeEach(func() {
		component = New(nil, "", Values{})
	})

	It("should successfully test the scrape config", func() {
		test.ScrapeConfigs(component, expectedScrapeConfig)
	})

	It("should successfully test the alerting rules", func() {
		test.AlertingRulesWithPromtool(
			component,
			map[string]string{"node-exporter.rules.yaml": expectedAlertingRule},
			filepath.Join("testdata", "monitoring_alertingrules.yaml"),
		)
	})
})

const (
	expectedScrapeConfig = `job_name: node-exporter
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
  api_server: https://kube-apiserver:443
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
  regex: node-exporter;metrics
# common metrics
- action: labelmap
  regex: __meta_kubernetes_service_label_(.+)
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
- source_labels: [ __meta_kubernetes_pod_node_name ]
  target_label: node
- target_label: __address__
  replacement: kube-apiserver:443
- source_labels: [__meta_kubernetes_pod_name, __meta_kubernetes_pod_container_port_number]
  regex: (.+);(.+)
  target_label: __metrics_path__
  replacement: /api/v1/namespaces/kube-system/pods/${1}:${2}/proxy/metrics
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(node_boot_time_seconds|node_cpu_seconds_total|node_disk_read_bytes_total|node_disk_written_bytes_total|node_disk_io_time_weighted_seconds_total|node_disk_io_time_seconds_total|node_disk_write_time_seconds_total|node_disk_writes_completed_total|node_disk_read_time_seconds_total|node_disk_reads_completed_total|node_filesystem_avail_bytes|node_filesystem_files|node_filesystem_files_free|node_filesystem_free_bytes|node_filesystem_readonly|node_filesystem_size_bytes|node_load1|node_load15|node_load5|node_memory_.+|node_nf_conntrack_.+|node_scrape_collector_duration_seconds|node_scrape_collector_success|process_max_fds|process_open_fds)$
`

	expectedAlertingRule = `groups:
- name: node-exporter.rules
  rules:
  - alert: NodeExporterDown
    expr: absent(up{job="node-exporter"} == 1)
    for: 1h
    labels:
      service: node-exporter
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
      service: node-exporter
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
      service: node-exporter
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
      service: node-exporter
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
      service: node-exporter
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
      service: node-exporter
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
