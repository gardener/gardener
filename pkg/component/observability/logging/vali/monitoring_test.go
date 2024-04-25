// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vali_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"

	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/observability/logging/vali"
	"github.com/gardener/gardener/pkg/component/test"
)

var _ = Describe("Monitoring", func() {
	var component component.MonitoringComponent

	BeforeEach(func() {
		component = New(nil, "test-namespace", nil, Values{})
	})

	It("should successfully test the scrape config", func() {
		test.ScrapeConfigs(component, expectedScrapeConfigVali)
	})

	It("should successfully test the scrape config when node logging is enabled", func() {
		component = New(nil, "test-namespace", nil, Values{
			ShootNodeLoggingEnabled: true,
		})
		test.ScrapeConfigs(component, expectedScrapeConfigVali, expectedScrapeConfigValiTelegraf)
	})

	It("should successfully test the alerting rules", func() {
		test.AlertingRulesWithPromtool(
			component,
			map[string]string{"vali.rules.yaml": expectedAlertingRule},
			filepath.Join("testdata", "monitoring_alertingrules.yaml"),
		)
	})
})

const (
	expectedScrapeConfigVali = `job_name: vali
honor_labels: false
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: [test-namespace]
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: logging;metrics
# common metrics
- action: labelmap
  regex: __meta_kubernetes_service_label_(.+)
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(vali_ingester_blocks_per_chunk_sum|vali_ingester_blocks_per_chunk_count|vali_ingester_chunk_age_seconds_sum|vali_ingester_chunk_age_seconds_count|vali_ingester_chunk_bounds_hours_sum|vali_ingester_chunk_bounds_hours_count|vali_ingester_chunk_compression_ratio_sum|vali_ingester_chunk_compression_ratio_count|vali_ingester_chunk_encode_time_seconds_sum|vali_ingester_chunk_encode_time_seconds_count|vali_ingester_chunk_entries_sum|vali_ingester_chunk_entries_count|vali_ingester_chunk_size_bytes_sum|vali_ingester_chunk_size_bytes_count|vali_ingester_chunk_utilization_sum|vali_ingester_chunk_utilization_count|vali_ingester_memory_chunks|vali_ingester_received_chunks|vali_ingester_samples_per_chunk_sum|vali_ingester_samples_per_chunk_count|vali_ingester_sent_chunks|vali_panic_total|vali_logql_querystats_duplicates_total|vali_logql_querystats_ingester_sent_lines_total)$
`

	expectedScrapeConfigValiTelegraf = `job_name: vali-telegraf
honor_labels: false
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: [test-namespace]
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: logging;telegraf
# common metrics
- action: labelmap
  regex: __meta_kubernetes_service_label_(.+)
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
metric_relabel_configs:
- source_labels: [__name__]
  target_label: __name__
  regex:  'iptables_(.+)'
  action: replace
  replacement: 'shoot_node_logging_incoming_$1'
`

	expectedAlertingRule = `groups:
- name: vali.rules
  rules:
  - alert: ValiDown
    expr: absent(up{job="vali"} == 1)
    for: 20m
    labels:
      service: vali
      severity: warning
      type: seed
      visibility: operator
    annotations:
      description: There are no running vali pods. No logs will be collected.
      summary: Vali is down
`
)
