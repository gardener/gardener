// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	registerOnce = make(chan struct{})

	flowTaskDelaySeconds    *prometheus.HistogramVec
	flowTaskDurationSeconds *prometheus.HistogramVec
	flowTaskResults         *prometheus.CounterVec
	flowDurationSeconds     *prometheus.HistogramVec
)

// Namespace is the metric namespace for the flow library.
const metricsNamespace = "flow"

// RegisterMetrics registers the metrics for the flow library on the passed registry.
// This function can only be called once.
// If this function is not called, no metrics are collected in this package.
func RegisterMetrics(r prometheus.Registerer) {
	close(registerOnce) // Metrics can only be registered once on a registry.

	factory := promauto.With(r)

	flowTaskDelaySeconds = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Name:      "task_delay_seconds",
			Help:      "Delay until invocation of a flow task.",
			Buckets:   prometheus.ExponentialBuckets(0.01, 3, 12),
		},
		[]string{
			"flow",
			"task_id",
			"skipped",
		},
	)

	flowTaskDurationSeconds = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Name:      "task_duration_seconds",
			Help:      "Duration of a flow task.",
			Buckets:   prometheus.ExponentialBuckets(0.01, 3, 12),
		},
		[]string{
			"flow",
			"task_id",
		},
	)

	flowTaskResults = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "task_results_total",
			Help:      "Flow task result counter. The value of the label 'result' can either be 'success' or 'error'.",
		},
		[]string{
			"flow",
			"task_id",
			"result",
		},
	)

	flowDurationSeconds = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Name:      "duration_seconds",
			Help:      "Total duration of a flow.",
			Buckets:   prometheus.ExponentialBuckets(0.02, 3, 12),
		},
		[]string{
			"flow",
		},
	)
}
