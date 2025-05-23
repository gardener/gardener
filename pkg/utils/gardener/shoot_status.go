// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// ShootStatus is the status of a shoot used in the common.ShootStatus label.
type ShootStatus string

const (
	// ShootStatusHealthy indicates that a shoot is considered healthy.
	ShootStatusHealthy ShootStatus = "healthy"
	// ShootStatusProgressing indicates that a shoot was once healthy, currently experienced an issue
	// but is still within a predefined grace period.
	ShootStatusProgressing ShootStatus = "progressing"
	// ShootStatusUnhealthy indicates that a shoot is considered unhealthy.
	ShootStatusUnhealthy ShootStatus = "unhealthy"
	// ShootStatusUnknown indicates that the shoot health status is not known.
	ShootStatusUnknown ShootStatus = "unknown"
)

var (
	shootStatusValues = map[ShootStatus]int{
		ShootStatusHealthy:     3,
		ShootStatusProgressing: 2,
		ShootStatusUnknown:     1,
		ShootStatusUnhealthy:   0,
	}
)

// ShootStatusValue returns the value of the given ShootStatus.
func ShootStatusValue(s ShootStatus) int {
	value, ok := shootStatusValues[s]
	if !ok {
		panic(fmt.Sprintf("invalid shoot status %q", s))
	}

	return value
}

// OrWorse returns the worse ShootStatus of the given two states.
func (s ShootStatus) OrWorse(other ShootStatus) ShootStatus {
	if ShootStatusValue(other) < ShootStatusValue(s) {
		return other
	}
	return s
}

// ConditionStatusToShootStatus converts the given ConditionStatus to a shoot label ShootStatus.
func ConditionStatusToShootStatus(status gardencorev1beta1.ConditionStatus) ShootStatus {
	switch status {
	case gardencorev1beta1.ConditionTrue:
		return ShootStatusHealthy
	case gardencorev1beta1.ConditionProgressing:
		return ShootStatusProgressing
	case gardencorev1beta1.ConditionUnknown:
		return ShootStatusUnknown
	}
	return ShootStatusUnhealthy
}

// ComputeConditionStatus computes the ShootStatus from the given Conditions. By default, the ShootStatus is
// ShootStatusHealthy. The condition status is converted to a ShootStatus by using ConditionStatusToShootStatus. Always
// the worst status of the combined states wins.
func ComputeConditionStatus(conditions ...gardencorev1beta1.Condition) ShootStatus {
	status := ShootStatusHealthy
	for _, condition := range conditions {
		status = status.OrWorse(ConditionStatusToShootStatus(condition.Status))
	}
	return status
}

// BoolToShootStatus converts the given boolean to a ShootStatus. For true values, it returns ShootStatusHealthy.
// Otherwise, it returns ShootStatusUnhealthy.
func BoolToShootStatus(cond bool) ShootStatus {
	if cond {
		return ShootStatusHealthy
	}
	return ShootStatusUnhealthy
}

// ComputeShootStatus computes the ShootStatus of a shoot depending on the given lastOperation, lastError and conditions.
func ComputeShootStatus(lastOperation *gardencorev1beta1.LastOperation, lastErrors []gardencorev1beta1.LastError, conditions ...gardencorev1beta1.Condition) ShootStatus {
	// Shoot has been created and not yet reconciled.
	if lastOperation == nil {
		return ShootStatusHealthy
	}

	// If the Shoot is either in create (except successful create) or delete state then the last error indicates the healthiness.
	if (lastOperation.Type == gardencorev1beta1.LastOperationTypeCreate && lastOperation.State != gardencorev1beta1.LastOperationStateSucceeded) ||
		lastOperation.Type == gardencorev1beta1.LastOperationTypeDelete {
		return BoolToShootStatus(len(lastErrors) == 0)
	}

	status := ComputeConditionStatus(conditions...)

	// If an operation is currently processing then the last error state is reported.
	if lastOperation.State == gardencorev1beta1.LastOperationStateProcessing {
		return status.OrWorse(BoolToShootStatus(len(lastErrors) == 0))
	}

	// If the last operation has succeeded then the shoot is healthy.
	return status.OrWorse(BoolToShootStatus(lastOperation.State == gardencorev1beta1.LastOperationStateSucceeded))
}
