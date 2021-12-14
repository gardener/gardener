// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shoot

import (
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// Status is the status of a shoot used in the common.ShootStatus label.
type Status string

const (
	// StatusHealthy indicates that a shoot is considered healthy.
	StatusHealthy Status = "healthy"
	// StatusProgressing indicates that a shoot was once healthy, currently experienced an issue
	// but is still within a predefined grace period.
	StatusProgressing Status = "progressing"
	// StatusUnhealthy indicates that a shoot is considered unhealthy.
	StatusUnhealthy Status = "unhealthy"
	// StatusUnknown indicates that the shoot health status is not known.
	StatusUnknown Status = "unknown"
)

var (
	shootStatusValues = map[Status]int{
		StatusHealthy:     3,
		StatusProgressing: 2,
		StatusUnknown:     1,
		StatusUnhealthy:   0,
	}
)

// StatusValue returns the value of the given Status.
func StatusValue(s Status) int {
	value, ok := shootStatusValues[s]
	if !ok {
		panic(fmt.Sprintf("invalid shoot status %q", s))
	}

	return value
}

// OrWorse returns the worse Status of the given two states.
func (s Status) OrWorse(other Status) Status {
	if StatusValue(other) < StatusValue(s) {
		return other
	}
	return s
}

// ConditionStatusToStatus converts the given ConditionStatus to a shoot label Status.
func ConditionStatusToStatus(status gardencorev1beta1.ConditionStatus) Status {
	switch status {
	case gardencorev1beta1.ConditionTrue:
		return StatusHealthy
	case gardencorev1beta1.ConditionProgressing:
		return StatusProgressing
	case gardencorev1beta1.ConditionUnknown:
		return StatusUnknown
	}
	return StatusUnhealthy
}

// ComputeConditionStatus computes a shoot Label Status from the given Conditions.
// By default, the Status is StatusHealthy. The condition status is converted to
// a Status by using ConditionStatusToStatus. Always the worst status of the combined
// states wins.
func ComputeConditionStatus(conditions ...gardencorev1beta1.Condition) Status {
	status := StatusHealthy
	for _, condition := range conditions {
		status = status.OrWorse(ConditionStatusToStatus(condition.Status))
	}
	return status
}

// BoolToStatus converts the given boolean to a Status.
// For true values, it returns StatusHealthy.
// Otherwise, it returns StatusUnhealthy.
func BoolToStatus(cond bool) Status {
	if cond {
		return StatusHealthy
	}
	return StatusUnhealthy
}

// ComputeStatus computes the label Status of a shoot depending on the given lastOperation, lastError and conditions.
func ComputeStatus(lastOperation *gardencorev1beta1.LastOperation, lastErrors []gardencorev1beta1.LastError, conditions ...gardencorev1beta1.Condition) Status {
	// Shoot has been created and not yet reconciled.
	if lastOperation == nil {
		return StatusHealthy
	}

	// If the Shoot is either in create (except successful create) or delete state then the last error indicates the healthiness.
	if (lastOperation.Type == gardencorev1beta1.LastOperationTypeCreate && lastOperation.State != gardencorev1beta1.LastOperationStateSucceeded) ||
		lastOperation.Type == gardencorev1beta1.LastOperationTypeDelete {
		return BoolToStatus(len(lastErrors) == 0)
	}

	status := ComputeConditionStatus(conditions...)

	// If an operation is currently processing then the last error state is reported.
	if lastOperation.State == gardencorev1beta1.LastOperationStateProcessing {
		return status.OrWorse(BoolToStatus(len(lastErrors) == 0))
	}

	// If the last operation has succeeded then the shoot is healthy.
	return status.OrWorse(BoolToStatus(lastOperation.State == gardencorev1beta1.LastOperationStateSucceeded))
}
