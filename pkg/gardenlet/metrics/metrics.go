// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	Factory = promauto.With(runtimemetrics.Registry)
	// ShootOperationTimings defines the histogram shoot_operation_duration_seconds.
	ShootOperationTimings = Factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Name:      "shoot_operation_duration_seconds",
			Help:      "Duration of shoot operations.",
			Buckets:   prometheus.LinearBuckets(180, 120, 10),
		},
		[]string{
			"operation",
			"workerless",
			"hibernated",
		},
	)
)
