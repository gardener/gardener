// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	runtimemetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Namespace is the metric namespace for the gardenlet.
const Namespace = "gardenlet"

var (
	// Factory is used for registering metrics in the controller-runtime metrics registry.
	factory = promauto.With(runtimemetrics.Registry)
	// ShootOperationDurationSeconds defines the histogram shoot_operation_duration_seconds.
	ShootOperationDurationSeconds = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Name:      "shoot_operation_duration_seconds",
			Help:      "Duration of shoot operations in seconds.",
			Buckets:   prometheus.LinearBuckets(180, 120, 10),
		},
		[]string{
			"operation",
			"workerless",
			"hibernated",
		},
	)
)
