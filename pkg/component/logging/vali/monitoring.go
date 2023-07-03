// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package vali

import (
	"bytes"
	"strings"
	"text/template"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

const (
	monitoringPrometheusJobNameVali         = "vali"
	monitoringPrometheusJobNameValiTelegraf = "vali-telegraf"

	monitoringMetricValiIngesterBlocksPerChunkSum             = "vali_ingester_blocks_per_chunk_sum"
	monitoringMetricValiIngesterBlocksPerChunkCount           = "vali_ingester_blocks_per_chunk_count"
	monitoringMetricValiIngesterChunkAgeSecondsSum            = "vali_ingester_chunk_age_seconds_sum"
	monitoringMetricValiIngesterChunkAgeSecondsCount          = "vali_ingester_chunk_age_seconds_count"
	monitoringMetricValiIngesterChunkBoundsHoursSum           = "vali_ingester_chunk_bounds_hours_sum"
	monitoringMetricValiIngesterChunkBoundsHoursCount         = "vali_ingester_chunk_bounds_hours_count"
	monitoringMetricValiIngesterChunkCompressionRatioSum      = "vali_ingester_chunk_compression_ratio_sum"
	monitoringMetricValiIngesterChunkCompressionRatioCount    = "vali_ingester_chunk_compression_ratio_count"
	monitoringMetricValiIngesterChunkEncodeTimeSecondsSum     = "vali_ingester_chunk_encode_time_seconds_sum"
	monitoringMetricValiIngesterChunkEncodeTimeSecondsCount   = "vali_ingester_chunk_encode_time_seconds_count"
	monitoringMetricValiIngesterChunkEntriesSum               = "vali_ingester_chunk_entries_sum"
	monitoringMetricValiIngesterChunkEntriesCount             = "vali_ingester_chunk_entries_count"
	monitoringMetricValiIngesterChunkSizeBytesSum             = "vali_ingester_chunk_size_bytes_sum"
	monitoringMetricValiIngesterChunkSizeBytesCount           = "vali_ingester_chunk_size_bytes_count"
	monitoringMetricValiIngesterChunkUtilizationSum           = "vali_ingester_chunk_utilization_sum"
	monitoringMetricValiIngesterChunkUtilizationCount         = "vali_ingester_chunk_utilization_count"
	monitoringMetricValiIngesterMemoryChunks                  = "vali_ingester_memory_chunks"
	monitoringMetricValiIngesterReceivedChunks                = "vali_ingester_received_chunks"
	monitoringMetricValiIngesterSamplesPerChunkSum            = "vali_ingester_samples_per_chunk_sum"
	monitoringMetricValiIngesterSamplesPerChunkCount          = "vali_ingester_samples_per_chunk_count"
	monitoringMetricValiIngesterSentChunks                    = "vali_ingester_sent_chunks"
	monitoringMetricValiPanicTotal                            = "vali_panic_total"
	monitoringMetricValiLogqlQuerystatsDuplicatesTotal        = "vali_logql_querystats_duplicates_total"
	monitoringMetricValiLogqlQuerystatsIngesterSentLinesTotal = "vali_logql_querystats_ingester_sent_lines_total"

	monitoringAlertingRules = `groups:
- name: vali.rules
  rules:
  - alert: ValiDown
    expr: absent(up{job="` + monitoringPrometheusJobNameVali + `"} == 1)
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

var (
	monitoringAllowedMetricsVali = []string{
		monitoringMetricValiIngesterBlocksPerChunkSum,
		monitoringMetricValiIngesterBlocksPerChunkCount,
		monitoringMetricValiIngesterChunkAgeSecondsSum,
		monitoringMetricValiIngesterChunkAgeSecondsCount,
		monitoringMetricValiIngesterChunkBoundsHoursSum,
		monitoringMetricValiIngesterChunkBoundsHoursCount,
		monitoringMetricValiIngesterChunkCompressionRatioSum,
		monitoringMetricValiIngesterChunkCompressionRatioCount,
		monitoringMetricValiIngesterChunkEncodeTimeSecondsSum,
		monitoringMetricValiIngesterChunkEncodeTimeSecondsCount,
		monitoringMetricValiIngesterChunkEntriesSum,
		monitoringMetricValiIngesterChunkEntriesCount,
		monitoringMetricValiIngesterChunkSizeBytesSum,
		monitoringMetricValiIngesterChunkSizeBytesCount,
		monitoringMetricValiIngesterChunkUtilizationSum,
		monitoringMetricValiIngesterChunkUtilizationCount,
		monitoringMetricValiIngesterMemoryChunks,
		monitoringMetricValiIngesterReceivedChunks,
		monitoringMetricValiIngesterSamplesPerChunkSum,
		monitoringMetricValiIngesterSamplesPerChunkCount,
		monitoringMetricValiIngesterSentChunks,
		monitoringMetricValiPanicTotal,
		monitoringMetricValiLogqlQuerystatsDuplicatesTotal,
		monitoringMetricValiLogqlQuerystatsIngesterSentLinesTotal,
	}

	monitoringScrapeConfigValiTmpl = `job_name: ` + monitoringPrometheusJobNameVali + `
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
  regex: logging;metrics
# common metrics
- action: labelmap
  regex: __meta_kubernetes_service_label_(.+)
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(` + strings.Join(monitoringAllowedMetricsVali, "|") + `)$
`

	monitoringScrapeConfigValiTelegrafTmpl = `job_name: ` + monitoringPrometheusJobNameValiTelegraf + `
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
	monitoringScrapeConfigValiTemplate, monitoringScrapeConfigValiTelegrafTemplate *template.Template
)

func init() {
	var err error

	monitoringScrapeConfigValiTemplate, err = template.New("monitoring-scrape-config-vali").Parse(monitoringScrapeConfigValiTmpl)
	utilruntime.Must(err)
	monitoringScrapeConfigValiTelegrafTemplate, err = template.New("monitoring-scrape-config-vali-telegraf").Parse(monitoringScrapeConfigValiTelegrafTmpl)
	utilruntime.Must(err)
}

// ScrapeConfigs returns the scrape configurations for vali.
func (v *vali) ScrapeConfigs() ([]string, error) {
	var scrapeConfigVali, scrapeConfigValiTelegraf bytes.Buffer

	if err := monitoringScrapeConfigValiTemplate.Execute(&scrapeConfigVali, map[string]interface{}{"namespace": v.namespace}); err != nil {
		return nil, err
	}

	if err := monitoringScrapeConfigValiTelegrafTemplate.Execute(&scrapeConfigValiTelegraf, map[string]interface{}{"namespace": v.namespace}); err != nil {
		return nil, err
	}

	if !v.values.KubeRBACProxyEnabled {
		return []string{scrapeConfigVali.String()}, nil
	}

	return []string{
		scrapeConfigVali.String(),
		scrapeConfigValiTelegraf.String(),
	}, nil
}

// AlertingRules returns the alerting rules for AlertManager.
func (v *vali) AlertingRules() (map[string]string, error) {
	return map[string]string{"vali.rules.yaml": monitoringAlertingRules}, nil
}
