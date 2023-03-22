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

package machinecontrollermanager_test

import (
	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo/v2"

	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/operation/botanist/component/test"
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
  regex: ^(mcm_cloud_api_requests_failed_total|mcm_cloud_api_requests_total|mcm_machine_controller_frozen|mcm_machine_current_status_phase|mcm_machine_deployment_failed_machines|mcm_machine_items_total|mcm_machine_set_failed_machines|mcm_machine_deployment_items_total|mcm_machine_set_items_total|mcm_scrape_failure_total|mcm_workqueue_adds_total|mcm_workqueue_depth|mcm_workqueue_queue_duration_seconds_bucket|mcm_workqueue_queue_duration_seconds_sum|mcm_workqueue_queue_duration_seconds_count|mcm_workqueue_work_duration_seconds_bucket|mcm_workqueue_work_duration_seconds_sum|mcm_workqueue_work_duration_seconds_count|mcm_workqueue_unfinished_work_seconds|mcm_workqueue_longest_running_processor_seconds|mcm_workqueue_retries_total|process_max_fds|process_open_fds)$
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
