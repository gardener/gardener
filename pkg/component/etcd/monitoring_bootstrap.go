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

package etcd

import (
	"strings"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
)

const (
	monitoringPrometheusJobName              = "etcddruid"
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
