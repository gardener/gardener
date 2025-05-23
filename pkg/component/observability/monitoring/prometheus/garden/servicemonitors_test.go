// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
)

var _ = Describe("ServiceMonitors", func() {
	Describe("#CentralServiceMonitors", func() {
		It("should return the expected objects", func() {
			Expect(garden.CentralServiceMonitors()).To(HaveExactElements(&monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{Name: "alertmanager-garden"},
				Spec: monitoringv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{MatchLabels: map[string]string{
						"component":    "alertmanager",
						"role":         "monitoring",
						"alertmanager": "garden",
					}},
					Endpoints: []monitoringv1.Endpoint{{
						Port: "metrics",
						MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
							SourceLabels: []monitoringv1.LabelName{"__name__"},
							Action:       "keep",
							Regex:        `^(alertmanager_alerts|alertmanager_alerts_received_total|alertmanager_build_info|alertmanager_cluster_health_score|alertmanager_cluster_members|alertmanager_cluster_peers_joined_total|alertmanager_config_hash|alertmanager_config_last_reload_success_timestamp_seconds|alertmanager_notifications_failed_total|alertmanager_notifications_total|alertmanager_peer_position|alertmanager_silences|process_cpu_seconds_total|process_resident_memory_bytes|process_start_time_seconds)$`,
						}},
					}},
				},
			}))
		})
	})
})
