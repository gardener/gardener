// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package helper

import (
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// ShouldPrepareShootForMigration determines whether the controller should prepare the shoot control plane for migration
// to another seed.
func ShouldPrepareShootForMigration(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.Status.SeedName != nil && shoot.Spec.SeedName != nil && *shoot.Spec.SeedName != *shoot.Status.SeedName
}

// ComputeOperationType determines which operation should be executed when acting on the given shoot.
func ComputeOperationType(shoot *gardencorev1beta1.Shoot) gardencorev1beta1.LastOperationType {
	if ShouldPrepareShootForMigration(shoot) {
		return gardencorev1beta1.LastOperationTypeMigrate
	}

	lastOperation := shoot.Status.LastOperation
	if lastOperation != nil && lastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate &&
		(lastOperation.State == gardencorev1beta1.LastOperationStateSucceeded || lastOperation.State == gardencorev1beta1.LastOperationStateAborted) {
		return gardencorev1beta1.LastOperationTypeRestore
	}

	return v1beta1helper.ComputeOperationType(shoot.ObjectMeta, shoot.Status.LastOperation)
}

// GetEtcdDeployTimeout returns the timeout for the etcd deployment task of the reconcile flow.
func GetEtcdDeployTimeout(shoot *shoot.Shoot, defaultDuration time.Duration) time.Duration {
	timeout := defaultDuration
	if v1beta1helper.IsHAControlPlaneConfigured(shoot.GetInfo()) {
		timeout = etcd.DefaultTimeout
	}
	return timeout
}

// IsSeedReadyForMigration checks if the seed can be used as a target seed for migrating a shoot control plane.
// If the seed is ready, it returns nil. Otherwise, it returns an error with a description.
func IsSeedReadyForMigration(seed *gardencorev1beta1.Seed, identity *gardencorev1beta1.Gardener) error {
	if seed.DeletionTimestamp != nil {
		return fmt.Errorf("seed is marked to be deleted")
	}
	return health.CheckSeedForMigration(seed, identity)
}
