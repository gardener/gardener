// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
