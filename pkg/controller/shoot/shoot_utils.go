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

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation/common"
	corev1 "k8s.io/api/core/v1"
)

func formatError(message string, err error) *gardenv1beta1.LastError {
	return &gardenv1beta1.LastError{
		Description: fmt.Sprintf("%s (%s)", message, err.Error()),
	}
}

func shootHealthyLabelTransform(healthy bool) func(*gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
	return func(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
		if shoot.Labels == nil {
			shoot.Labels = make(map[string]string)
		}

		if !healthy {
			shoot.Labels[common.ShootUnhealthy] = "true"
		} else {
			delete(shoot.Labels, common.ShootUnhealthy)
		}

		return shoot, nil
	}
}

func mustIgnoreShoot(annotations map[string]string, respectSyncPeriodOverwrite *bool) bool {
	_, ignore := annotations[common.ShootIgnore]
	return respectSyncPeriodOverwrite != nil && *respectSyncPeriodOverwrite && ignore
}

func shootIsFailed(shoot *gardenv1beta1.Shoot) bool {
	lastOperation := shoot.Status.LastOperation
	return lastOperation != nil && lastOperation.State == gardenv1beta1.ShootLastOperationStateFailed && shoot.Generation == shoot.Status.ObservedGeneration
}

func shootIsHealthy(shoot *gardenv1beta1.Shoot, conditions ...*gardenv1beta1.Condition) bool {
	var (
		lastOperation = shoot.Status.LastOperation
		lastError     = shoot.Status.LastError

		allConditionsTrue = func() bool {
			for _, condition := range conditions {
				if condition.Status != corev1.ConditionTrue {
					return false
				}
			}
			return true
		}
	)

	// Shoot has been created and not yet reconciled.
	if lastOperation == nil {
		return true
	}

	// If shoot is created or deleted then the last error indicates the healthiness.
	if lastOperation.Type == gardenv1beta1.ShootLastOperationTypeCreate || lastOperation.Type == gardenv1beta1.ShootLastOperationTypeDelete {
		return lastError == nil
	}

	// If the shoot is normally reconciled then at least one false condition indicates that something is wrong.
	if !allConditionsTrue() {
		return false
	}

	// If an operation is currently processing then the last error state is reported.
	if lastOperation.State == gardenv1beta1.ShootLastOperationStateProcessing {
		return lastError == nil
	}

	// If the last operation has succeeded then the shoot is healthy.
	return lastOperation.State == gardenv1beta1.ShootLastOperationStateSucceeded
}

func seedIsShoot(seed *gardenv1beta1.Seed) bool {
	hasOwnerReference, _ := seedHasShootOwnerReference(seed.ObjectMeta)
	return hasOwnerReference
}

func shootIsSeed(shoot *gardenv1beta1.Shoot) bool {
	shootedSeed, err := helper.ReadShootedSeed(shoot)
	return err == nil && shootedSeed != nil
}
