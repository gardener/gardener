// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	runtimemetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Namespace is the metric namespace for the gardener-admission-controller.
const Namespace = "gardener_admission_controller"

var (
	// Factory is used for registering metrics in the controller-runtime metrics registry.
	Factory = promauto.With(runtimemetrics.Registry)

	// RejectedResources defines the counter rejected_resources_total.
	RejectedResources = Factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "rejected_resources_total",
			Help:      "Total number of resources rejected.",
		},
		[]string{
			"operation",
			"kind",
			"namespace",
			"reason",
		},
	)
)
