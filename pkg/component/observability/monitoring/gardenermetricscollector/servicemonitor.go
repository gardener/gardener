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
	openTelemetryCollectorServiceTypeLabel      = "operator.opentelemetry.io/collector-service-type"
	openTelemetryCollectorBaseServiceType       = "base"
	openTelemetryCollectorMonitoringServiceType = "monitoring"
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

func (g *gardenerMetricsCollector) internalMetricsServiceMonitor() *monitoringv1.ServiceMonitor {
	selectorLabels := getLabels()
	selectorLabels[openTelemetryCollectorServiceTypeLabel] = openTelemetryCollectorMonitoringServiceType

	allowedMetrics := []string{
		"otelcol_exporter_enqueue_failed_metric_points",
		"otelcol_exporter_queue_capacity",
		"otelcol_exporter_queue_size",
		"otelcol_exporter_send_failed_metric_points",
		"otelcol_exporter_sent_metric_points",
		"otelcol_process_cpu_seconds",
		"otelcol_process_cpu_seconds_total",
		"otelcol_process_memory_rss",
		"otelcol_process_memory_rss_bytes",
		"otelcol_process_runtime_heap_alloc_bytes",
		"otelcol_process_runtime_total_alloc_bytes_total",
		"otelcol_process_runtime_total_sys_memory_bytes",
		"otelcol_process_uptime",
		"otelcol_process_uptime_seconds_total",
		"otelcol_processor_incoming_items",
		"otelcol_processor_incoming_items_total",
		"otelcol_processor_outgoing_items",
		"otelcol_processor_outgoing_items_total",
		"otelcol_receiver_accepted_metric_points",
		"otelcol_receiver_refused_metric_points",
		"otelcol_scraper_errored_metric_points",
		"otelcol_scraper_scraped_metric_points",
	}

	return &monitoringv1.ServiceMonitor{
		ObjectMeta: monitoringutils.ConfigObjectMeta(openTelemetryCollectorName+"-monitoring", g.namespace, garden.Label),
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: selectorLabels},
			Endpoints: []monitoringv1.Endpoint{{
				// The OpenTelemetry Operator names the monitoring service's port "monitoring".
				Port: "monitoring",
				RelabelConfigs: []monitoringv1.RelabelConfig{
					{
						Action:      "replace",
						Replacement: new(openTelemetryCollectorName),
						TargetLabel: "job",
					},
				},
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(allowedMetrics...),
			}},
		},
	}
}
