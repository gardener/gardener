// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubestatemetrics_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/kubestatemetrics"
	"github.com/gardener/gardener/pkg/component/test"
)

var _ = Describe("Monitoring", func() {
	Context("Shoot Monitoring Configuration", func() {
		var kubeStateMetrics component.MonitoringComponent

		BeforeEach(func() {
			kubeStateMetrics = New(nil, "shoot--foo--bar", nil, Values{})
		})

		It("should successfully test the scrape configs", func() {
			test.ScrapeConfigs(kubeStateMetrics, expectedScrapeConfig)
		})

		It("should successfully test the alerting rules", func() {
			test.AlertingRulesWithPromtool(
				kubeStateMetrics,
				map[string]string{"kube-state-metrics.rules.yaml": expectedAlertingRules},
				filepath.Join("testdata", "monitoring_alertingrules.yaml"),
			)
		})
	})

	Context("Shoot Monitoring Configuration (Workerless Shoot)", func() {
		var kubeStateMetrics component.MonitoringComponent

		BeforeEach(func() {
			kubeStateMetrics = New(nil, "shoot--foo--bar", nil, Values{IsWorkerless: true})
		})

		It("should return empty for scrape configs", func() {
			scrapeConfigs, err := kubeStateMetrics.ScrapeConfigs()
			Expect(err).NotTo(HaveOccurred())

			Expect(scrapeConfigs).To(BeEmpty())
		})

		It("should successfully test the alerting rules", func() {
			test.AlertingRulesWithPromtool(
				kubeStateMetrics,
				map[string]string{"kube-state-metrics.rules.yaml": expectedAlertingRulesWorkerlessShoot},
				filepath.Join("testdata", "monitoring_alertingrules_workerless.yaml"),
			)
		})
	})
})

const (
	expectedScrapeConfig = `job_name: kube-state-metrics
honor_labels: false
# Service is used, because we only care about metric from one kube-state-metrics instance
# and not multiple in HA setup
kubernetes_sd_configs:
- role: service
  namespaces:
    names: [ shoot--foo--bar ]
relabel_configs:
- source_labels: [ __meta_kubernetes_service_label_component ]
  action: keep
  regex: kube-state-metrics
- source_labels: [ __meta_kubernetes_service_port_name ]
  action: keep
- source_labels: [ __meta_kubernetes_service_label_type ]
  regex: (.+)
  target_label: type
  replacement: ${1}
- target_label: instance
  replacement: kube-state-metrics
metric_relabel_configs:
- source_labels: [ pod ]
  regex: ^.+\.tf-pod.+$
  action: drop
- source_labels: [ __name__ ]
  action: keep
  regex: ^(kube_daemonset_metadata_generation|kube_daemonset_status_current_number_scheduled|kube_daemonset_status_desired_number_scheduled|kube_daemonset_status_number_available|kube_daemonset_status_number_unavailable|kube_daemonset_status_updated_number_scheduled|kube_deployment_metadata_generation|kube_deployment_spec_replicas|kube_deployment_status_observed_generation|kube_deployment_status_replicas|kube_deployment_status_replicas_available|kube_deployment_status_replicas_unavailable|kube_deployment_status_replicas_updated|kube_node_info|kube_node_labels|kube_node_spec_taint|kube_node_spec_unschedulable|kube_node_status_allocatable|kube_node_status_capacity|kube_node_status_condition|kube_pod_container_info|kube_pod_container_resource_limits|kube_pod_container_resource_requests|kube_pod_container_status_restarts_total|kube_pod_info|kube_pod_labels|kube_pod_status_phase|kube_pod_status_ready|kube_replicaset_metadata_generation|kube_replicaset_owner|kube_replicaset_spec_replicas|kube_replicaset_status_observed_generation|kube_replicaset_status_replicas|kube_replicaset_status_ready_replicas|kube_statefulset_metadata_generation|kube_statefulset_replicas|kube_statefulset_status_observed_generation|kube_statefulset_status_replicas|kube_statefulset_status_replicas_current|kube_statefulset_status_replicas_ready|kube_statefulset_status_replicas_updated|kube_verticalpodautoscaler_status_recommendation_containerrecommendations_target|kube_verticalpodautoscaler_status_recommendation_containerrecommendations_upperbound|kube_verticalpodautoscaler_status_recommendation_containerrecommendations_lowerbound|kube_verticalpodautoscaler_spec_resourcepolicy_container_policies_minallowed|kube_verticalpodautoscaler_spec_resourcepolicy_container_policies_maxallowed|kube_verticalpodautoscaler_spec_updatepolicy_updatemode)$
`

	expectedAlertingRules = `groups:
- name: kube-state-metrics.rules
  rules:
  - alert: KubeStateMetricsShootDown
    expr: absent(up{job="kube-state-metrics", type="shoot"} == 1)
    for: 15m
    labels:
      service: kube-state-metrics-shoot
      severity: info
      visibility: operator
      type: seed
    annotations:
      summary: Kube-state-metrics for shoot cluster metrics is down.
      description: There are no running kube-state-metric pods for the shoot cluster. No kubernetes resource metrics can be scraped.

  - alert: KubeStateMetricsSeedDown
    expr: absent(count({exported_job="kube-state-metrics"}))
    for: 15m
    labels:
      service: kube-state-metrics-seed
      severity: critical
      visibility: operator
      type: seed
    annotations:
      summary: There are no kube-state-metrics metrics for the control plane
      description: Kube-state-metrics is scraped by the cache prometheus and federated by the control plane prometheus. Something is broken in that process.

  - alert: NoWorkerNodes
    expr: sum(kube_node_spec_unschedulable) == count(kube_node_info) or absent(kube_node_info)
    for: 25m # MCM timeout + grace period to allow self healing before firing alert
    labels:
      service: nodes
      severity: blocker
      visibility: all
    annotations:
      description: There are no worker nodes in the cluster or all of the worker nodes in the cluster are not schedulable.
      summary: No nodes available. Possibly all workloads down.

  - record: shoot:kube_node_status_capacity_cpu_cores:sum
    expr: sum(kube_node_status_capacity{resource="cpu",unit="core"})

  - record: shoot:kube_node_status_capacity_memory_bytes:sum
    expr: sum(kube_node_status_capacity{resource="memory",unit="byte"})

  - record: shoot:machine_types:sum
    expr: sum(kube_node_labels) by (label_beta_kubernetes_io_instance_type)

  - record: shoot:node_operating_system:sum
    expr: sum(kube_node_info) by (os_image, kernel_version)

  # Mitigation for extension dashboards.
  # TODO(istvanballok): Remove in a future version. For more details, see https://github.com/gardener/gardener/pull/6224.
  - record: kube_pod_container_resource_limits_cpu_cores
    expr: kube_pod_container_resource_limits{resource="cpu", unit="core"}

  - record: kube_pod_container_resource_requests_cpu_cores
    expr: kube_pod_container_resource_requests{resource="cpu", unit="core"}

  - record: kube_pod_container_resource_limits_memory_bytes
    expr: kube_pod_container_resource_limits{resource="memory", unit="byte"}

  - record: kube_pod_container_resource_requests_memory_bytes
    expr: kube_pod_container_resource_requests{resource="memory", unit="byte"}
`

	expectedAlertingRulesWorkerlessShoot = `groups:
- name: kube-state-metrics.rules
  rules:
  - alert: KubeStateMetricsSeedDown
    expr: absent(count({exported_job="kube-state-metrics"}))
    for: 15m
    labels:
      service: kube-state-metrics-seed
      severity: critical
      visibility: operator
      type: seed
    annotations:
      summary: There are no kube-state-metrics metrics for the control plane
      description: Kube-state-metrics is scraped by the cache prometheus and federated by the control plane prometheus. Something is broken in that process.
`
)
