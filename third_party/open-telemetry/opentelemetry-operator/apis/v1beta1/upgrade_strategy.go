// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Important:
// This file is copied from https://github.com/open-telemetry/opentelemetry-operator/blob/v0.143.0/apis/v1beta1/upgrade_strategy.go.

package v1beta1

type (
	// UpgradeStrategy represents how the operator will handle upgrades to the CR when a newer version of the operator is deployed
	// +kubebuilder:validation:Enum=automatic;none
	UpgradeStrategy string
)

const (
	// UpgradeStrategyAutomatic specifies that the operator will automatically apply upgrades to the CR.
	UpgradeStrategyAutomatic UpgradeStrategy = "automatic"

	// UpgradeStrategyNone specifies that the operator will not apply any upgrades to the CR.
	UpgradeStrategyNone UpgradeStrategy = "none"
)
