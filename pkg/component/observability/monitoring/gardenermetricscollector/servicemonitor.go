// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenermetricscollector

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

const (
	openTelemetryCollectorServiceTypeLabel = "operator.opentelemetry.io/collector-service-type"
	openTelemetryCollectorBaseServiceType  = "base"
)

func (g *gardenerMetricsCollector) serviceMonitor() *monitoringv1.ServiceMonitor {
	selectorLabels := GetLabels()
	selectorLabels[openTelemetryCollectorServiceTypeLabel] = openTelemetryCollectorBaseServiceType

	return &monitoringv1.ServiceMonitor{
		ObjectMeta: monitoringutils.ConfigObjectMeta(openTelemetryCollectorName, g.namespace, garden.Label),
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: selectorLabels},
			Endpoints: []monitoringv1.Endpoint{{
				Port: metricsPortName,
				RelabelConfigs: []monitoringv1.RelabelConfig{
					// The OpenTelemetry Operator creates the Service after the collector resource. Without explicitly
					// overriding the job label, prometheus-operator would choose the service name as the job.
					{
						Action:      "replace",
						Replacement: new(openTelemetryCollectorName),
						TargetLabel: "job",
					},
				},
				// TODO: The metrics exposed by the Gardener metrics receiver differ slightly from those of the
				// gardener-metrics-exporter. For now, keep all scraped metrics without filtering. Once the metric set
				// has stabilized, add a StandardMetricRelabelConfig allowlist here.
			}},
		},
	}
}
