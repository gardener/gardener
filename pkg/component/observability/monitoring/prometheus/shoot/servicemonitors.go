// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	_ "embed"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/alertmanager"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

// CentralServiceMonitors returns the central ServiceMonitor resources for the shoot prometheus.
func CentralServiceMonitors(wantsAlertmanager bool) []*monitoringv1.ServiceMonitor {
	var out []*monitoringv1.ServiceMonitor

	if wantsAlertmanager {
		out = append(out, &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{Name: "alertmanager-shoot"},
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: alertmanager.GetLabels("shoot")},
				Endpoints: []monitoringv1.Endpoint{{
					Port: alertmanager.PortNameMetrics,
					RelabelConfigs: []monitoringv1.RelabelConfig{{
						Action: "labelmap",
						Regex:  `__meta_kubernetes_service_label_(.+)`,
					}},
					MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
						"alertmanager_config_hash",
						"alertmanager_config_last_reload_successful",
						"process_max_fds",
						"process_open_fds",
					),
				}},
			},
		})
	}

	return out
}
