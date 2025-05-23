// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package discoveryserver

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

func (g *gardenerDiscoveryServer) serviceMonitor() *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		ObjectMeta: monitoringutils.ConfigObjectMeta(deploymentName, g.namespace, garden.Label),
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: labels()},
			Endpoints: []monitoringv1.Endpoint{{
				Port: portNameMetrics,
				RelabelConfigs: []monitoringv1.RelabelConfig{{
					Action: "labelmap",
					Regex:  `__meta_kubernetes_service_label_(.+)`,
				}},
			}},
		},
	}
}
