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

package controllerutils

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SeedFilterFunc returns a filtering func for seeds.
func SeedFilterFunc(seedName string) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		seed, ok := obj.(*gardencorev1beta1.Seed)
		if !ok {
			return false
		}

		return seed.Name == seedName
	}
}

// ShootFilterFunc returns a filtering func for shoots.
func ShootFilterFunc(seedName string) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		shoot, ok := obj.(*gardencorev1beta1.Shoot)
		if !ok {
			return false
		}
		if shoot.Spec.SeedName == nil {
			return false
		}

		if shoot.Status.SeedName == nil || *shoot.Spec.SeedName == *shoot.Status.SeedName {
			return *shoot.Spec.SeedName == seedName
		}

		return *shoot.Status.SeedName == seedName
	}
}

// ShootMigrationFilterFunc returns a filtering func for shoots that are being migrated to a different seed.
func ShootMigrationFilterFunc(ctx context.Context, c client.Reader, seedName string) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		shoot, ok := obj.(*gardencorev1beta1.Shoot)
		if !ok {
			return false
		}

		return ShootIsBeingMigratedToSeed(ctx, c, shoot, seedName)
	}
}

// ShootIsBeingMigratedToSeed checks if the given shoot is currently being migrated to the seed with the given name,
// and the source seed has ownerChecks enabled (as it is a prerequisite to successfully force restore a shoot to a different seed).
func ShootIsBeingMigratedToSeed(ctx context.Context, c client.Reader, shoot *gardencorev1beta1.Shoot, seedName string) bool {
	if shoot.Spec.SeedName != nil && shoot.Status.SeedName != nil && *shoot.Spec.SeedName != *shoot.Status.SeedName && *shoot.Spec.SeedName == seedName {
		seed := &gardencorev1beta1.Seed{}
		if err := c.Get(ctx, kutil.Key(*shoot.Status.SeedName), seed); err != nil {
			return false
		}
		return gardencorev1beta1helper.SeedSettingOwnerChecksEnabled(seed.Spec.Settings)
	}
	return false
}

// BackupBucketFilterFunc returns a filtering func for BackupBuckets.
func BackupBucketFilterFunc(seedName string) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
		if !ok {
			return false
		}
		if backupBucket.Spec.SeedName == nil {
			return false
		}

		return *backupBucket.Spec.SeedName == seedName
	}
}

// BackupEntryFilterFunc returns a filtering func for BackupEntries.
func BackupEntryFilterFunc(seedName string) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
		if !ok {
			return false
		}
		if backupEntry.Spec.SeedName == nil {
			return false
		}

		if backupEntry.Status.SeedName == nil || *backupEntry.Spec.SeedName == *backupEntry.Status.SeedName {
			return *backupEntry.Spec.SeedName == seedName
		}

		return *backupEntry.Status.SeedName == seedName
	}
}

// BackupEntryMigrationFilterFunc returns a filtering func for backup entries that are being migrated to a different seed.
func BackupEntryMigrationFilterFunc(ctx context.Context, c client.Reader, seedName string) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
		if !ok {
			return false
		}

		return BackupEntryIsBeingMigratedToSeed(ctx, c, backupEntry, seedName)
	}
}

// BackupEntryIsBeingMigratedToSeed checks if the given BackupEntry is currently being migrated to the seed with the given name,
// and the source seed has ownerChecks enabled (as it is a prerequisite to successfully force restore a shoot to a different seed).
func BackupEntryIsBeingMigratedToSeed(ctx context.Context, c client.Reader, backupEntry *gardencorev1beta1.BackupEntry, seedName string) bool {
	if backupEntry.Spec.SeedName != nil && backupEntry.Status.SeedName != nil && *backupEntry.Spec.SeedName != *backupEntry.Status.SeedName && *backupEntry.Spec.SeedName == seedName {
		seed := &gardencorev1beta1.Seed{}
		if err := c.Get(ctx, kutil.Key(*backupEntry.Status.SeedName), seed); err != nil {
			return false
		}
		return gardencorev1beta1helper.SeedSettingOwnerChecksEnabled(seed.Spec.Settings)
	}
	return false
}

// BastionFilterFunc returns a filtering func for Bastions.
func BastionFilterFunc(seedName string) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		bastion, ok := obj.(*operationsv1alpha1.Bastion)
		if !ok {
			return false
		}
		if bastion.Spec.SeedName == nil {
			return false
		}

		return *bastion.Spec.SeedName == seedName
	}
}

// ManagedSeedFilterFunc returns a filtering func for ManagedSeeds that checks if the ManagedSeed references a Shoot scheduled on a Seed, for which the gardenlet is responsible..
func ManagedSeedFilterFunc(ctx context.Context, c client.Reader, seedName string) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		managedSeed, ok := obj.(*seedmanagementv1alpha1.ManagedSeed)
		if !ok {
			return false
		}
		if managedSeed.Spec.Shoot == nil || managedSeed.Spec.Shoot.Name == "" {
			return false
		}
		shoot := &gardencorev1beta1.Shoot{}
		if err := c.Get(ctx, kutil.Key(managedSeed.Namespace, managedSeed.Spec.Shoot.Name), shoot); err != nil {
			return false
		}
		if shoot.Spec.SeedName == nil {
			return false
		}

		if shoot.Status.SeedName == nil || *shoot.Spec.SeedName == *shoot.Status.SeedName {
			return *shoot.Spec.SeedName == seedName
		}

		return *shoot.Status.SeedName == seedName
	}
}

// SeedOfManagedSeedFilterFunc returns a filtering func for Seeds that checks if the Seed is owned by a ManagedSeed
// that references a Shoot scheduled on a Seed, for which the gardenlet is responsible.
func SeedOfManagedSeedFilterFunc(ctx context.Context, c client.Reader, seedName string) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		seed, ok := obj.(*gardencorev1beta1.Seed)
		if !ok {
			return false
		}
		managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
		if err := c.Get(ctx, kutil.Key(gardencorev1beta1constants.GardenNamespace, seed.Name), managedSeed); err != nil {
			return false
		}
		return ManagedSeedFilterFunc(ctx, c, seedName)(managedSeed)
	}
}
