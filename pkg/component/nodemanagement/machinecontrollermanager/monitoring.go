// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
)

// Machine Deployment related metrics
const (
	monitoringMetricMCMMachineDeploymentItemsTotal                          = "mcm_machine_deployment_items_total"
	monitoringMetricMCMMachineDeploymentInfo                                = "mcm_machine_deployment_info"
	monitoringMetricMCMMachineDeploymentInfoSpecPaused                      = "mcm_machine_deployment_info_spec_paused"
	monitoringMetricMCMMachineDeploymentInfoSpecReplicas                    = "mcm_machine_deployment_info_spec_replicas"
	monitoringMetricMCMMachineDeploymentInfoSpecMinReadySeconds             = "mcm_machine_deployment_info_spec_min_ready_seconds"
	monitoringMetricMCMMachineDeploymentInfoSpecRollingUpdateMaxSurge       = "mcm_machine_deployment_info_spec_rolling_update_max_surge"
	monitoringMetricMCMMachineDeploymentInfoSpecRollingUpdateMaxUnavailable = "mcm_machine_deployment_info_spec_rolling_update_max_unavailable"
	monitoringMetricMCMMachineDeploymentInfoSpecRevisionHistoryLimit        = "mcm_machine_deployment_info_spec_revision_history_limit"
	monitoringMetricMCMMachineDeploymentInfoSpecProgressDeadlineSeconds     = "mcm_machine_deployment_info_spec_progress_deadline_seconds"
	monitoringMetricMCMMachineDeploymentInfoSpecRollbackToRevision          = "mcm_machine_deployment_info_spec_rollback_to_revision"
	monitoringMetricMCMMachineDeploymentStatusCondition                     = "mcm_machine_deployment_status_condition"
	monitoringMetricMCMMachineDeploymentStatusAvailableReplicas             = "mcm_machine_deployment_status_available_replicas"
	monitoringMetricMCMMachineDeploymentStatusUnavailableReplicas           = "mcm_machine_deployment_status_unavailable_replicas"
	monitoringMetricMCMMachineDeploymentStatusReadyReplicas                 = "mcm_machine_deployment_status_ready_replicas"
	monitoringMetricMCMMachineDeploymentStatusUpdatedReplicas               = "mcm_machine_deployment_status_updated_replicas"
	monitoringMetricMCMMachineDeploymentStatusCollisionCount                = "mcm_machine_deployment_status_collision_count"
	monitoringMetricMCMMachineDeploymentStatusReplicas                      = "mcm_machine_deployment_status_replicas"
	monitoringMetricMCMMachineDeploymentFailedMachines                      = "mcm_machine_deployment_failed_machines"
)

// Machine Set related metrics
const (
	monitoringMetricMCMMachineSetItemsTotal                  = "mcm_machine_set_items_total"
	monitoringMetricMCMMachineSetInfo                        = "mcm_machine_set_info"
	monitoringMetricMCMMachineSetFailedMachines              = "mcm_machine_set_failed_machines"
	monitoringMetricMCMMachineSetInfoSpecReplicas            = "mcm_machine_set_info_spec_replicas"
	monitoringMetricMCMMachineSetInfoSpecMinReadySeconds     = "mcm_machine_set_info_spec_min_ready_seconds"
	monitoringMetricMCMMachineSetStatusCondition             = "mcm_machine_set_status_condition"
	monitoringMetricMCMMachineSetStatusAvailableReplicas     = "mcm_machine_set_status_available_replicas"
	monitoringMetricMCMMachineSetStatusFullyLabelledReplicas = "mcm_machine_set_status_fully_labelled_replicas"
	monitoringMetricMCMMachineSetStatusReplicas              = "mcm_machine_set_status_replicas"
	monitoringMetricMCMMachineSetStatusReadyReplicas         = "mcm_machine_set_status_ready_replicas"
)

// Machine related metrics
const (
	monitoringMetricMCMMachineStaleMachinesTotal = "mcm_machine_stale_machines_total"
	monitoringMetricMCMMachineItemsTotal         = "mcm_machine_items_total"
	monitoringMetricMCMMachineCurrentStatusPhase = "mcm_machine_current_status_phase"
	monitoringMetricMCMMachineInfo               = "mcm_machine_info"
	monitoringMetricMCMMachineStatusCondition    = "mcm_machine_status_condition"
)

// Cloud API related metrics
const (
	monitoringMetricMCMCloudAPIRequestsTotal                      = "mcm_cloud_api_requests_total"
	monitoringMetricMCMCloudAPIRequestsFailedTotal                = "mcm_cloud_api_requests_failed_total"
	monitoringMetricMCMCloudAPIRequestDurationSecondsBucket       = "mcm_cloud_api_api_request_duration_seconds_bucket"
	monitoringMetricMCMCloudAPIRequestDurationSecondsSum          = "mcm_cloud_api_api_request_duration_seconds_sum"
	monitoringMetricMCMCloudAPIRequestDurationSecondsCount        = "mcm_cloud_api_api_request_duration_seconds_count"
	monitoringMetricMCMCloudAPIDriverRequestDurationSecondsSum    = "mcm_cloud_api_driver_request_duration_seconds_sum"
	monitoringMetricMCMCloudAPIDriverRequestDurationSecondsCount  = "mcm_cloud_api_driver_request_duration_seconds_count"
	monitoringMetricMCMCloudAPIDriverRequestDurationSecondsBucket = "mcm_cloud_api_driver_request_duration_seconds_bucket"
	monitoringMetricMCMCloudAPIDriverRequestFailedTotal           = "mcm_cloud_api_driver_request_failed_total"
)

// misc metrics
const (
	monitoringMetricMCMScrapeFailureTotal      = "mcm_misc_scrape_failure_total"
	monitoringMetricMCMMachineControllerFrozen = "mcm_machine_controller_frozen"
	monitoringMetricProcessMaxFds              = "process_max_fds"
	monitoringMetricProcessOpenFds             = "process_open_fds"
)

// workqueue related metrics
const (
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
)

// alert rules
const (
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
		monitoringMetricMCMMachineDeploymentItemsTotal,
		monitoringMetricMCMMachineDeploymentInfo,
		monitoringMetricMCMMachineDeploymentInfoSpecPaused,
		monitoringMetricMCMMachineDeploymentInfoSpecReplicas,
		monitoringMetricMCMMachineDeploymentInfoSpecMinReadySeconds,
		monitoringMetricMCMMachineDeploymentInfoSpecRollingUpdateMaxSurge,
		monitoringMetricMCMMachineDeploymentInfoSpecRollingUpdateMaxUnavailable,
		monitoringMetricMCMMachineDeploymentInfoSpecRevisionHistoryLimit,
		monitoringMetricMCMMachineDeploymentInfoSpecProgressDeadlineSeconds,
		monitoringMetricMCMMachineDeploymentInfoSpecRollbackToRevision,
		monitoringMetricMCMMachineDeploymentStatusCondition,
		monitoringMetricMCMMachineDeploymentStatusAvailableReplicas,
		monitoringMetricMCMMachineDeploymentStatusUnavailableReplicas,
		monitoringMetricMCMMachineDeploymentStatusReadyReplicas,
		monitoringMetricMCMMachineDeploymentStatusUpdatedReplicas,
		monitoringMetricMCMMachineDeploymentStatusCollisionCount,
		monitoringMetricMCMMachineDeploymentStatusReplicas,
		monitoringMetricMCMMachineDeploymentFailedMachines,

		monitoringMetricMCMMachineSetInfo,
		monitoringMetricMCMMachineSetInfoSpecReplicas,
		monitoringMetricMCMMachineSetInfoSpecMinReadySeconds,
		monitoringMetricMCMMachineSetItemsTotal,
		monitoringMetricMCMMachineSetFailedMachines,
		monitoringMetricMCMMachineSetStatusCondition,
		monitoringMetricMCMMachineSetStatusAvailableReplicas,
		monitoringMetricMCMMachineSetStatusFullyLabelledReplicas,
		monitoringMetricMCMMachineSetStatusReplicas,
		monitoringMetricMCMMachineSetStatusReadyReplicas,

		monitoringMetricMCMMachineStaleMachinesTotal,
		monitoringMetricMCMMachineItemsTotal,
		monitoringMetricMCMMachineCurrentStatusPhase,
		monitoringMetricMCMMachineInfo,
		monitoringMetricMCMMachineStatusCondition,

		monitoringMetricMCMCloudAPIRequestsTotal,
		monitoringMetricMCMCloudAPIRequestsFailedTotal,
		monitoringMetricMCMCloudAPIRequestDurationSecondsBucket,
		monitoringMetricMCMCloudAPIRequestDurationSecondsSum,
		monitoringMetricMCMCloudAPIRequestDurationSecondsCount,
		monitoringMetricMCMCloudAPIDriverRequestDurationSecondsSum,
		monitoringMetricMCMCloudAPIDriverRequestDurationSecondsCount,
		monitoringMetricMCMCloudAPIDriverRequestDurationSecondsBucket,
		monitoringMetricMCMCloudAPIDriverRequestFailedTotal,

		monitoringMetricMCMMachineControllerFrozen,
		monitoringMetricMCMScrapeFailureTotal,
		monitoringMetricProcessMaxFds,
		monitoringMetricProcessOpenFds,

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
