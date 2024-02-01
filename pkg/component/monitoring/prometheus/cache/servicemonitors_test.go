// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package cache_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/component/monitoring/prometheus/cache"
	monitoringutils "github.com/gardener/gardener/pkg/component/monitoring/utils"
)

var _ = Describe("ServiceMonitors", func() {
	Describe("#CentralServiceMonitors", func() {
		It("should return the expected objects", func() {
			Expect(cache.CentralServiceMonitors()).To(HaveExactElements(&monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node-exporter",
					Namespace: "kube-system",
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{MatchLabels: map[string]string{"component": "node-exporter"}},
					Endpoints: []monitoringv1.Endpoint{{
						Port: "metrics",
						RelabelConfigs: []*monitoringv1.RelabelConfig{
							{
								Action: "labelmap",
								Regex:  `__meta_kubernetes_service_label_(.+)`,
							},
							{
								SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_node_name"},
								TargetLabel:  "node",
							},
						},
						MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
							"node_boot_time_seconds",
							"node_cpu_seconds_total",
							"node_filesystem_avail_bytes",
							"node_filesystem_files",
							"node_filesystem_files_free",
							"node_filesystem_free_bytes",
							"node_filesystem_readonly",
							"node_filesystem_size_bytes",
							"node_load1",
							"node_load5",
							"node_load15",
							"node_memory_.+",
							"node_nf_conntrack_entries",
							"node_nf_conntrack_entries_limit",
							"process_max_fds",
							"process_open_fds",
						),
					}},
				},
			}))
		})
	})
})
