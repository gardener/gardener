// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package perses

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

func (p *perses) serviceMonitor() *monitoringv1.ServiceMonitor {
	prometheusLabel := "seed"
	if p.values.IsGardenCluster {
		prometheusLabel = "garden"
	}

	return &monitoringv1.ServiceMonitor{
		ObjectMeta: monitoringutils.ConfigObjectMeta(labelApp, p.namespace, prometheusLabel),
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: map[string]string{
				"app.kubernetes.io/instance": p.persesName(),
			}},
			Endpoints: []monitoringv1.Endpoint{{
				TargetPort: new(intstr.FromInt32(8080)),
			}},
		},
	}
}
