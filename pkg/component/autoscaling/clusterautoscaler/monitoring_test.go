// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package clusterautoscaler_test

import (
	. "github.com/onsi/ginkgo/v2"

	. "github.com/gardener/gardener/pkg/component/autoscaling/clusterautoscaler"
	"github.com/gardener/gardener/pkg/component/test"
)

var _ = Describe("Monitoring", func() {
	clusterAutoscaler := New(nil, "", nil, "", 0, nil, 0, nil)

	Describe("#ScrapeConfig", func() {
		It("should successfully test the scrape configuration", func() {
			test.ScrapeConfigs(clusterAutoscaler, expectedScrapeConfig)
		})
	})

	Describe("#AlertingRules", func() {
		It("should successfully test the alerting rules", func() {
			test.AlertingRules(clusterAutoscaler, map[string]string{"cluster-autoscaler.rules.yaml": expectedAlertingRule})
		})
	})
})

const (
	expectedScrapeConfig = `job_name: cluster-autoscaler
honor_labels: false
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: []
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: cluster-autoscaler;metrics
- action: labelmap
  regex: __meta_kubernetes_service_label_(.+)
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(process_max_fds|process_open_fds|cluster_autoscaler_cluster_safe_to_autoscale|cluster_autoscaler_nodes_count|cluster_autoscaler_unschedulable_pods_count|cluster_autoscaler_node_groups_count|cluster_autoscaler_max_nodes_count|cluster_autoscaler_cluster_cpu_current_cores|cluster_autoscaler_cpu_limits_cores|cluster_autoscaler_cluster_memory_current_bytes|cluster_autoscaler_memory_limits_bytes|cluster_autoscaler_last_activity|cluster_autoscaler_function_duration_seconds|cluster_autoscaler_errors_total|cluster_autoscaler_scaled_up_nodes_total|cluster_autoscaler_scaled_down_nodes_total|cluster_autoscaler_scaled_up_gpu_nodes_total|cluster_autoscaler_scaled_down_gpu_nodes_total|cluster_autoscaler_failed_scale_ups_total|cluster_autoscaler_evicted_pods_total|cluster_autoscaler_unneeded_nodes_count|cluster_autoscaler_old_unregistered_nodes_removed_count|cluster_autoscaler_skipped_scale_events_count)$
`

	expectedAlertingRule = `groups:
- name: cluster-autoscaler.rules
  rules:
  - alert: ClusterAutoscalerDown
    expr: absent(up{job="cluster-autoscaler"} == 1)
    for: 7m
    labels:
      service: cluster-autoscaler
      severity: critical
      type: seed
    annotations:
      description: There is no running cluster autoscaler. Shoot's Nodes wont be scaled dynamically, based on the load.
      summary: Cluster autoscaler is down
`
)
