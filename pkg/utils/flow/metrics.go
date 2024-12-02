// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	registerOnce = make(chan struct{})

	flowTaskTimingsDelay    *prometheus.HistogramVec
	flowTaskTimingsDuration *prometheus.HistogramVec
	flowTaskTotalDuration   *prometheus.HistogramVec
	flowTaskResults         *prometheus.CounterVec
)

// Namespace is the metric namespace for the flow library.
const Namespace = "flow"

// RegisterMetrics registers the metrics for the flow library on the passed registry.
func RegisterMetrics(r prometheus.Registerer) {
	close(registerOnce) // Metrics can only be registered once on a registry.

	factory := promauto.With(r)

	// flowTaskTimingsDelay defines the histogram to record the delay in seconds from flow start until task start.
	flowTaskTimingsDelay = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Name:      "task_delay_seconds",
			Help:      "Delay until invocation of a flow task.",
			Buckets:   prometheus.ExponentialBuckets(0.01, 3, 12),
		},
		[]string{
			"flow",
			"taskid",
			"skipped",
		},
	)

	// flowTaskTimingsDuration defines the histogram to record the duration in seconds of a flow task.
	flowTaskTimingsDuration = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Name:      "task_duration_seconds",
			Help:      "Duration of a flow task.",
			Buckets:   prometheus.ExponentialBuckets(0.01, 3, 12),
		},
		[]string{
			"flow",
			"taskid",
		},
	)

	// flowTaskTotalDuration defines the histogram to record the total duration in seconds of a flow.
	flowTaskTotalDuration = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Name:      "total_duration_seconds",
			Help:      "Total duration of a flow.",
			Buckets:   prometheus.ExponentialBuckets(0.02, 3, 12),
		},
		[]string{
			"flow",
		},
	)

	// flowTaskErrors defines the histogram to record errors occurred during execution of a flow task.
	flowTaskResults = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "task_result_total",
			Help:      "Flow task result counter.",
		},
		[]string{
			"flow",
			"taskid",
			"result",
		},
	)
}
