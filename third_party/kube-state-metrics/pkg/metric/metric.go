// Copyright The Kubernetes Authors
// SPDX-License-Identifier: Apache-2.0

// Important:
// This file is copied from https://github.com/kubernetes/kube-state-metrics/blob/v2.19.0/pkg/metric/metric.go.
// Only the Type definition and constants used by Gardener are included.

package metric

// Type represents the type of the metric. See https://github.com/prometheus/OpenMetrics/blob/v1.0.0/specification/OpenMetrics.md#metric-types.
type Type string

// Supported metric types.
var (
	// Gauge defines an OpenMetrics gauge.
	Gauge Type = "gauge"

	// Info defines an OpenMetrics info.
	Info Type = "info"

	// StateSet defines an OpenMetrics stateset.
	StateSet Type = "stateset"

	// Counter defines an OpenMetrics counter.
	Counter Type = "counter"
)
