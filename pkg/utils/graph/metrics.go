// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gardener/gardener/pkg/admissioncontroller/metrics"
)

const seedAuthorizerSubsystem = "seed_authorizer"

var (
	metricUpdateDuration = metrics.Factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: seedAuthorizerSubsystem,
			Namespace: metrics.Namespace,
			Name:      "graph_update_duration_seconds",
			Help:      "Histogram of duration of resource dependency graph updates in seed authorizer.",
			// Start with 0.1ms with the last bucket being [~200ms, Inf)
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 12),
		},
		[]string{
			"kind",
			"operation",
		},
	)

	metricPathCheckDuration = metrics.Factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: seedAuthorizerSubsystem,
			Namespace: metrics.Namespace,
			Name:      "graph_path_check_duration_seconds",
			Help:      "Histogram of duration of checks whether a path exists in the resource dependency graph in seed authorizer.",
			// Start with 0.1ms with the last bucket being [~200ms, Inf)
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 12),
		},
		[]string{
			"fromKind",
			"toKind",
		},
	)
)
