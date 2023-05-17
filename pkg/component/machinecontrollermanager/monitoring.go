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

package machinecontrollermanager

import (
	"bytes"
	"strings"
	"text/template"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

const (
	monitoringPrometheusJobName = "machine-controller-manager"

	monitoringMetricMCMCloudAPIRequestsFailedTotal             = "mcm_cloud_api_requests_failed_total"
	monitoringMetricMCMCloudAPIRequestsTotal                   = "mcm_cloud_api_requests_total"
	monitoringMetricMCMMachineControllerFrozen                 = "mcm_machine_controller_frozen"
	monitoringMetricMCMMachineCurrentStatusPhase               = "mcm_machine_current_status_phase"
	monitoringMetricMCMMachineDeploymentFailedMachines         = "mcm_machine_deployment_failed_machines"
	monitoringMetricMCMMachineItemsTotal                       = "mcm_machine_items_total"
	monitoringMetricMCMMachineSetFailedMachines                = "mcm_machine_set_failed_machines"
	monitoringMetricMCMMachineDeploymentItemsTotal             = "mcm_machine_deployment_items_total"
	monitoringMetricMCMMachineSetItemsTotal                    = "mcm_machine_set_items_total"
	monitoringMetricMCMMachineSetStaleMachinesTotal            = "mcm_machine_set_stale_machines_total"
	monitoringMetricMCMScrapeFailureTotal                      = "mcm_scrape_failure_total"
	monitoringMetricMCMWorkqueueAddsTotal                      = "mcm_workqueue_adds_total"
	monitoringMetricMCMWorkqueueDepth                          = "mcm_workqueue_depth"
	monitoringMetricMCMWorkqueueQueueDurationSecondsBucket     = "mcm_workqueue_queue_duration_seconds_bucket"
	monitoringMetricMCMWorkqueueQueueDurationSecondsSum        = "mcm_workqueue_queue_duration_seconds_sum"
	monitoringMetricMCMWorkqueueQueueDurationSecondsCount      = "mcm_workqueue_queue_duration_seconds_count"
	monitoringMetricMCMWorkqueueWorkDurationSecondsBucket      = "mcm_workqueue_work_duration_seconds_bucket"
	monitoringMetricMCMWorkqueueWorkDurationSecondsSum         = "mcm_workqueue_work_duration_seconds_sum"
	monitoringMetricMCMWorkqueueWorkDurationSecondsCount       = "mcm_workqueue_work_duration_seconds_count"
	monitoringMetricMCMWorkqueueUnfinishedWorkSeconds          = "mcm_workqueue_unfinished_work_seconds"
	monitoringMetricMCMWorkqueueLongestRunningProcessorSeconds = "mcm_workqueue_longest_running_processor_seconds"
	monitoringMetricMCMWorkqueueRetriesTotal                   = "mcm_workqueue_retries_total"
	monitoringMetricProcessMaxFds                              = "process_max_fds"
	monitoringMetricProcessOpenFds                             = "process_open_fds"

	monitoringAlertingRules = `groups:
- name: machine-controller-manager.rules
  rules:
  - alert: MachineControllerManagerDown
    expr: absent(up{job="` + monitoringPrometheusJobName + `"} == 1)
    for: 15m
    labels:
      service: ` + v1beta1constants.DeploymentNameMachineControllerManager + `
      severity: critical
      type: seed
      visibility: operator
    annotations:
      description: There are no running machine controller manager instances. No shoot nodes can be created/maintained.
      summary: Machine controller manager is down.
`
)

var (
	monitoringAllowedMetrics = []string{
		monitoringMetricMCMCloudAPIRequestsFailedTotal,
		monitoringMetricMCMCloudAPIRequestsTotal,
		monitoringMetricMCMMachineControllerFrozen,
		monitoringMetricMCMMachineCurrentStatusPhase,
		monitoringMetricMCMMachineDeploymentFailedMachines,
		monitoringMetricMCMMachineItemsTotal,
		monitoringMetricMCMMachineSetFailedMachines,
		monitoringMetricMCMMachineDeploymentItemsTotal,
		monitoringMetricMCMMachineSetItemsTotal,
		monitoringMetricMCMMachineSetStaleMachinesTotal,
		monitoringMetricMCMScrapeFailureTotal,
		monitoringMetricMCMWorkqueueAddsTotal,
		monitoringMetricMCMWorkqueueDepth,
		monitoringMetricMCMWorkqueueQueueDurationSecondsBucket,
		monitoringMetricMCMWorkqueueQueueDurationSecondsSum,
		monitoringMetricMCMWorkqueueQueueDurationSecondsCount,
		monitoringMetricMCMWorkqueueWorkDurationSecondsBucket,
		monitoringMetricMCMWorkqueueWorkDurationSecondsSum,
		monitoringMetricMCMWorkqueueWorkDurationSecondsCount,
		monitoringMetricMCMWorkqueueUnfinishedWorkSeconds,
		monitoringMetricMCMWorkqueueLongestRunningProcessorSeconds,
		monitoringMetricMCMWorkqueueRetriesTotal,
		monitoringMetricProcessMaxFds,
		monitoringMetricProcessOpenFds,
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
  regex: ` + serviceName + `;` + portNameMetrics + `
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
func (m *machineControllerManager) ScrapeConfigs() ([]string, error) {
	var scrapeConfig bytes.Buffer

	if err := monitoringScrapeConfigTemplate.Execute(&scrapeConfig, map[string]interface{}{"namespace": m.namespace}); err != nil {
		return nil, err
	}

	return []string{scrapeConfig.String()}, nil
}

// AlertingRules returns the alerting rules for AlertManager.
func (m *machineControllerManager) AlertingRules() (map[string]string, error) {
	return map[string]string{"machine-controller-manager.rules.yaml": monitoringAlertingRules}, nil
}
