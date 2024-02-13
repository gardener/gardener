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

package seed

import (
	_ "embed"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	monitoringutils "github.com/gardener/gardener/pkg/component/monitoring/utils"
)

// CentralScrapeConfigs returns the central ScrapeConfig resources for the seed prometheus.
func CentralScrapeConfigs() []*monitoringv1alpha1.ScrapeConfig {
	return []*monitoringv1alpha1.ScrapeConfig{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "prometheus",
			},
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				RelabelConfigs: []*monitoringv1.RelabelConfig{{
					Action:      "replace",
					Replacement: "prometheus",
					TargetLabel: "job",
				}},
				StaticConfigs: []monitoringv1alpha1.StaticConfig{{
					Targets: []monitoringv1alpha1.Target{"localhost:9090"},
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cadvisor",
			},
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				HonorLabels:     ptr.To(true),
				HonorTimestamps: ptr.To(false),
				MetricsPath:     ptr.To("/federate"),
				Params: map[string][]string{
					"match[]": {
						`{job="cadvisor",namespace=~"extension-(.+)"}`,
						`{job="cadvisor",namespace="garden"}`,
						`{job="cadvisor",namespace=~"istio-(.+)"}`,
						`{job="cadvisor",namespace="kube-system"}`,
					},
				},
				StaticConfigs: []monitoringv1alpha1.StaticConfig{{
					Targets: []monitoringv1alpha1.Target{"prometheus-cache.garden.svc"},
				}},
				RelabelConfigs: []*monitoringv1.RelabelConfig{{
					Action:      "replace",
					Replacement: "cadvisor",
					TargetLabel: "job",
				}},
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
					"container_cpu_cfs_periods_total",
					"container_cpu_cfs_throttled_periods_total",
					"container_cpu_cfs_throttled_seconds_total",
					"container_cpu_usage_seconds_total",
					"container_fs_inodes_total",
					"container_fs_limit_bytes",
					"container_fs_usage_bytes",
					"container_last_seen",
					"container_memory_working_set_bytes",
					"container_network_receive_bytes_total",
					"container_network_transmit_bytes_total",
					"container_oom_events_total",
				),
			},
		},
	}
}
