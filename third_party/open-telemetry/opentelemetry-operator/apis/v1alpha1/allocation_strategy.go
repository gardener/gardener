// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Important:
// This file is copied from https://github.com/open-telemetry/opentelemetry-operator/blob/v0.143.0/apis/v1alpha1/allocation_strategy.go.

package v1alpha1

type (
	// OpenTelemetryTargetAllocatorAllocationStrategy represent which strategy to distribute target to each collector
	// +kubebuilder:validation:Enum=least-weighted;consistent-hashing;per-node
	OpenTelemetryTargetAllocatorAllocationStrategy string
)

const (
	// OpenTelemetryTargetAllocatorAllocationStrategyLeastWeighted targets will be distributed to collector with fewer targets currently assigned.
	OpenTelemetryTargetAllocatorAllocationStrategyLeastWeighted OpenTelemetryTargetAllocatorAllocationStrategy = "least-weighted"

	// OpenTelemetryTargetAllocatorAllocationStrategyConsistentHashing targets will be consistently added to collectors, which allows a high-availability setup.
	OpenTelemetryTargetAllocatorAllocationStrategyConsistentHashing OpenTelemetryTargetAllocatorAllocationStrategy = "consistent-hashing"

	// OpenTelemetryTargetAllocatorAllocationStrategyPerNode targets will be assigned to the collector on the node they reside on (use only with daemon set).
	OpenTelemetryTargetAllocatorAllocationStrategyPerNode OpenTelemetryTargetAllocatorAllocationStrategy = "per-node"
)
