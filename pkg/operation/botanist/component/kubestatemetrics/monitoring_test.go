// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	. "github.com/gardener/gardener/pkg/operation/botanist/component/kubestatemetrics"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Monitoring", func() {
	Describe("#CentralMonitoringConfiguration", func() {
		It("should return the expected scrape configs", func() {
			monitoringConfig, err := CentralMonitoringConfiguration()

			Expect(err).NotTo(HaveOccurred())
			Expect(monitoringConfig.ScrapeConfigs).To(ConsistOf(`job_name: kube-state-metrics
honor_labels: false
# Service is used, because we only care about metric from one kube-state-metrics instance
# and not multiple in HA setup
kubernetes_sd_configs:
- role: service
  namespaces:
    names: [ garden ]
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
  regex: ^(kube_persistentvolumeclaim_resource_requests_storage_bytes|kube_daemonset_metadata_generation|kube_daemonset_status_current_number_scheduled|kube_daemonset_status_desired_number_scheduled|kube_daemonset_status_number_available|kube_daemonset_status_number_unavailable|kube_daemonset_status_updated_number_scheduled|kube_deployment_metadata_generation|kube_deployment_spec_replicas|kube_deployment_status_observed_generation|kube_deployment_status_replicas|kube_deployment_status_replicas_available|kube_deployment_status_replicas_unavailable|kube_deployment_status_replicas_updated|kube_horizontalpodautoscaler_spec_max_replicas|kube_horizontalpodautoscaler_spec_min_replicas|kube_horizontalpodautoscaler_status_current_replicas|kube_horizontalpodautoscaler_status_desired_replicas|kube_horizontalpodautoscaler_status_condition|kube_node_info|kube_node_labels|kube_node_spec_unschedulable|kube_node_status_allocatable|kube_node_status_capacity|kube_node_status_condition|kube_pod_container_info|kube_pod_container_resource_limits|kube_pod_container_resource_requests|kube_pod_container_status_restarts_total|kube_pod_info|kube_pod_labels|kube_pod_status_phase|kube_pod_status_ready|kube_statefulset_metadata_generation|kube_statefulset_replicas|kube_statefulset_status_observed_generation|kube_statefulset_status_replicas|kube_statefulset_status_replicas_current|kube_statefulset_status_replicas_ready|kube_statefulset_status_replicas_updated)$
`))
			Expect(monitoringConfig.CAdvisorScrapeConfigMetricRelabelConfigs).To(BeEmpty())
		})
	})
})
