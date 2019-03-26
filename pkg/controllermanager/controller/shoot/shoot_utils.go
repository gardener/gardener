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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/kubernetes"
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
)

var (
	shootStatusValues = map[Status]int{
		StatusHealthy:     3,
		StatusProgressing: 2,
		StatusUnhealthy:   1,
	}
)

func statusToBool(status Status) bool {
	return status == StatusHealthy
}

func statusValue(s Status) int {
	value, ok := shootStatusValues[s]
	if !ok {
		panic(fmt.Sprintf("invalid shoot status %q", s))
	}

	return value
}

// OrWorse returns the worse Status of the given two states.
func (s Status) OrWorse(other Status) Status {
	if statusValue(other) < statusValue(s) {
		return other
	}
	return s
}

func formatError(message string, err error) *gardencorev1alpha1.LastError {
	return &gardencorev1alpha1.LastError{
		Description: fmt.Sprintf("%s (%s)", message, err.Error()),
	}
}

// StatusLabelTransform transforms the shoot labels depending on the given Status.
func StatusLabelTransform(status Status) func(*gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
	return func(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
		// TODO (AC): Deprecate common.ShootUnhealthy once tools adapted
		healthy := statusToBool(status)
		if !healthy {
			kubernetes.SetMetaDataLabel(&shoot.ObjectMeta, common.ShootUnhealthy, "true")
		} else {
			delete(shoot.Labels, common.ShootUnhealthy)
		}
		kubernetes.SetMetaDataLabel(&shoot.ObjectMeta, common.ShootStatus, string(status))
		return shoot, nil
	}
}

func mustIgnoreShoot(annotations map[string]string, respectSyncPeriodOverwrite *bool) bool {
	_, ignore := annotations[common.ShootIgnore]
	return respectSyncPeriodOverwrite != nil && *respectSyncPeriodOverwrite && ignore
}

func shootIsFailed(shoot *gardenv1beta1.Shoot) bool {
	lastOperation := shoot.Status.LastOperation
	return lastOperation != nil && lastOperation.State == gardencorev1alpha1.LastOperationStateFailed && shoot.Generation == shoot.Status.ObservedGeneration
}

// ConditionStatusToStatus converts the given ConditionStatus to a shoot label Status.
func ConditionStatusToStatus(status gardencorev1alpha1.ConditionStatus) Status {
	switch status {
	case gardencorev1alpha1.ConditionTrue:
		return StatusHealthy
	case gardencorev1alpha1.ConditionProgressing:
		return StatusProgressing
	}
	return StatusUnhealthy
}

// ComputeConditionStatus computes a shoot Label Status from the given Conditions.
// By default, the Status is StatusHealthy. The condition status is converted to
// a Status by using ConditionStatusToStatus. Always the worst status of the combined
// states wins.
func ComputeConditionStatus(conditions ...gardencorev1alpha1.Condition) Status {
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
func ComputeStatus(lastOperation *gardencorev1alpha1.LastOperation, lastError *gardencorev1alpha1.LastError, conditions ...gardencorev1alpha1.Condition) Status {
	// Shoot has been created and not yet reconciled.
	if lastOperation == nil {
		return StatusHealthy
	}

	// If shoot is created or deleted then the last error indicates the healthiness.
	if lastOperation.Type == gardencorev1alpha1.LastOperationTypeCreate || lastOperation.Type == gardencorev1alpha1.LastOperationTypeDelete {
		return BoolToStatus(lastError == nil)
	}

	status := ComputeConditionStatus(conditions...)

	// If an operation is currently processing then the last error state is reported.
	if lastOperation.State == gardencorev1alpha1.LastOperationStateProcessing {
		return status.OrWorse(BoolToStatus(lastError == nil))
	}

	// If the last operation has succeeded then the shoot is healthy.
	return status.OrWorse(BoolToStatus(lastOperation.State == gardencorev1alpha1.LastOperationStateSucceeded))
}

func seedIsShoot(seed *gardenv1beta1.Seed) bool {
	hasOwnerReference, _ := seedHasShootOwnerReference(seed.ObjectMeta)
	return hasOwnerReference
}

func shootIsSeed(shoot *gardenv1beta1.Shoot) bool {
	shootedSeed, err := helper.ReadShootedSeed(shoot)
	return err == nil && shootedSeed != nil
}
