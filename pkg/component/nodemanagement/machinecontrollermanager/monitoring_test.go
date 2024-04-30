// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package machinecontrollermanager_test

import (
	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"

	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/component/test"
)

var _ = Describe("Monitoring", func() {
	var mcm component.MonitoringComponent

	BeforeEach(func() {
		mcm = New(nil, "some-namespace", nil, Values{RuntimeKubernetesVersion: semver.MustParse("1.26.1")})
	})

	Describe("#ScrapeConfig", func() {
		It("should successfully test the scrape configuration", func() {
			test.ScrapeConfigs(mcm, expectedScrapeConfig)
		})
	})

	Describe("#AlertingRules", func() {
		It("should successfully test the alerting rules", func() {
			test.AlertingRules(mcm, map[string]string{"machine-controller-manager.rules.yaml": expectedAlertingRule})
		})
	})
})

const (
	expectedScrapeConfig = `job_name: machine-controller-manager
honor_labels: false
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: [some-namespace]
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: machine-controller-manager;metrics
- action: labelmap
  regex: __meta_kubernetes_service_label_(.+)
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(mcm_machine_deployment_items_total|mcm_machine_deployment_info|mcm_machine_deployment_info_spec_paused|mcm_machine_deployment_info_spec_replicas|mcm_machine_deployment_info_spec_min_ready_seconds|mcm_machine_deployment_info_spec_rolling_update_max_surge|mcm_machine_deployment_info_spec_rolling_update_max_unavailable|mcm_machine_deployment_info_spec_revision_history_limit|mcm_machine_deployment_info_spec_progress_deadline_seconds|mcm_machine_deployment_info_spec_rollback_to_revision|mcm_machine_deployment_status_condition|mcm_machine_deployment_status_available_replicas|mcm_machine_deployment_status_unavailable_replicas|mcm_machine_deployment_status_ready_replicas|mcm_machine_deployment_status_updated_replicas|mcm_machine_deployment_status_collision_count|mcm_machine_deployment_status_replicas|mcm_machine_deployment_failed_machines|mcm_machine_set_info|mcm_machine_set_info_spec_replicas|mcm_machine_set_info_spec_min_ready_seconds|mcm_machine_set_items_total|mcm_machine_set_failed_machines|mcm_machine_set_status_condition|mcm_machine_set_status_available_replicas|mcm_machine_set_status_fully_labelled_replicas|mcm_machine_set_status_replicas|mcm_machine_set_status_ready_replicas|mcm_machine_stale_machines_total|mcm_machine_items_total|mcm_machine_current_status_phase|mcm_machine_info|mcm_machine_status_condition|mcm_cloud_api_requests_total|mcm_cloud_api_requests_failed_total|mcm_cloud_api_api_request_duration_seconds_bucket|mcm_cloud_api_api_request_duration_seconds_sum|mcm_cloud_api_api_request_duration_seconds_count|mcm_cloud_api_driver_request_duration_seconds_sum|mcm_cloud_api_driver_request_duration_seconds_count|mcm_cloud_api_driver_request_duration_seconds_bucket|mcm_cloud_api_driver_request_failed_total|mcm_machine_controller_frozen|mcm_misc_scrape_failure_total|process_max_fds|process_open_fds|mcm_workqueue_adds_total|mcm_workqueue_depth|mcm_workqueue_queue_duration_seconds_bucket|mcm_workqueue_queue_duration_seconds_sum|mcm_workqueue_queue_duration_seconds_count|mcm_workqueue_work_duration_seconds_bucket|mcm_workqueue_work_duration_seconds_sum|mcm_workqueue_work_duration_seconds_count|mcm_workqueue_unfinished_work_seconds|mcm_workqueue_longest_running_processor_seconds|mcm_workqueue_retries_total)$
`

	expectedAlertingRule = `groups:
- name: machine-controller-manager.rules
  rules:
  - alert: MachineControllerManagerDown
    expr: absent(up{job="machine-controller-manager"} == 1)
    for: 15m
    labels:
      service: machine-controller-manager
      severity: critical
      type: seed
      visibility: operator
    annotations:
      description: There are no running machine controller manager instances. No shoot nodes can be created/maintained.
      summary: Machine controller manager is down.
`
)
