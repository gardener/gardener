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

package garden

import (
	_ "embed"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/alertmanager"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

// CentralServiceMonitors returns the central ServiceMonitor resources for the garden prometheus.
func CentralServiceMonitors() []*monitoringv1.ServiceMonitor {
	return []*monitoringv1.ServiceMonitor{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "alertmanager-garden"},
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: alertmanager.GetLabels("garden")},
				Endpoints: []monitoringv1.Endpoint{{
					Port: alertmanager.PortNameMetrics,
					MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
						"alertmanager_alerts",
						"alertmanager_alerts_received_total",
						"alertmanager_build_info",
						"alertmanager_cluster_health_score",
						"alertmanager_cluster_members",
						"alertmanager_cluster_peers_joined_total",
						"alertmanager_config_hash",
						"alertmanager_config_last_reload_success_timestamp_seconds",
						"alertmanager_notifications_failed_total",
						"alertmanager_notifications_total",
						"alertmanager_peer_position",
						"alertmanager_silences",
						"process_cpu_seconds_total",
						"process_resident_memory_bytes",
						"process_start_time_seconds",
					),
				}},
			},
		},
	}
}
