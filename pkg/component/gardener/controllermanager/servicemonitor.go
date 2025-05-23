// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllermanager

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

func (g *gardenerControllerManager) serviceMonitor() *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		ObjectMeta: monitoringutils.ConfigObjectMeta(DeploymentName, g.namespace, garden.Label),
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: GetLabels()},
			Endpoints: []monitoringv1.Endpoint{{
				Port: portNameMetrics,
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
					"rest_client_.+",
					"controller_runtime_.+",
					"workqueue_.+",
					"go_.+",
				),
			}},
		},
	}
}
