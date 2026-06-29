// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pvcautoscaler

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

// Values configures the PersistentVolumeClaimAutoscaler resource that observability
// components (Vali, VictoriaLogs, Prometheus) create for their PVCs.
type Values struct {
	// Enabled controls whether the component creates a PersistentVolumeClaimAutoscaler resource.
	Enabled bool
	// MaxCapacity is the upper bound up to which the PVC may be scaled.
	MaxCapacity resource.Quantity
	// UtilizationThresholdPercent is the used-space/inodes threshold at which scaling is triggered.
	UtilizationThresholdPercent *int
	// StepPercent is the percentage by which the PVC capacity is changed on each scale operation.
	StepPercent *int
	// MinStepAbsolute is the minimum absolute capacity change applied per scale operation.
	MinStepAbsolute *resource.Quantity
}
