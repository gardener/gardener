// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"strings"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
)

const (
	monitoringPrometheusJobName              = "etcd-druid"
	monitoringMetricJobsTotal                = "etcddruid_compaction_jobs_total"
	monitoringMetricJobsCurrent              = "etcddruid_compaction_jobs_current"
	monitoringMetricJobDurationSecondsBucket = "etcddruid_compaction_job_duration_seconds_bucket"
	monitoringMetricJobDurationSecondsSum    = "etcddruid_compaction_job_duration_seconds_sum"
	monitoringMetricJobDurationSecondsCount  = "etcddruid_compaction_job_duration_seconds_count"
	monitoringMetricNumDeltaEvents           = "etcddruid_compaction_num_delta_events"

	serviceName     = "etcd-druid"
	portNameMetrics = "metrics"
)

var (
	monitoringAllowedMetrics = []string{
		monitoringMetricJobsTotal,
		monitoringMetricJobsCurrent,
		monitoringMetricJobDurationSecondsBucket,
		monitoringMetricJobDurationSecondsSum,
		monitoringMetricJobDurationSecondsCount,
		monitoringMetricNumDeltaEvents,
	}

	monitoringScrapeConfig = `job_name: ` + monitoringPrometheusJobName + `
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: [ ` + v1beta1constants.GardenNamespace + ` ]
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: ` + serviceName + `;` + portNameMetrics + `
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(` + strings.Join(monitoringAllowedMetrics, "|") + `)$
`
)

// CentralMonitoringConfiguration returns scrape configs for the central Prometheus.
func CentralMonitoringConfiguration() (component.CentralMonitoringConfig, error) {
	return component.CentralMonitoringConfig{ScrapeConfigs: []string{monitoringScrapeConfig}}, nil
}
