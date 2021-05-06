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
	gardenoperationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LabelsMatchFor checks whether the given label selector matches for the given set of labels.
func LabelsMatchFor(l map[string]string, labelSelector *metav1.LabelSelector) bool {
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return false
	}
	return selector.Matches(labels.Set(l))
}

// SeedFilterFunc returns a filtering func for the seeds and the given label selector.
func SeedFilterFunc(seedName string, labelSelector *metav1.LabelSelector) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		seed, ok := obj.(*gardencorev1beta1.Seed)
		if !ok {
			return false
		}
		if len(seedName) > 0 {
			return seed.Name == seedName
		}
		return LabelsMatchFor(seed.Labels, labelSelector)
	}
}

// ShootFilterFunc returns a filtering func for the seeds and the given label selector.
func ShootFilterFunc(seedName string, seedLister gardencorelisters.SeedLister, labelSelector *metav1.LabelSelector) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		shoot, ok := obj.(*gardencorev1beta1.Shoot)
		if !ok {
			return false
		}
		if shoot.Spec.SeedName == nil {
			return false
		}
		if len(seedName) > 0 {
			if shoot.Status.SeedName == nil || *shoot.Spec.SeedName == *shoot.Status.SeedName {
				return *shoot.Spec.SeedName == seedName
			}
			return *shoot.Status.SeedName == seedName
		}
		if shoot.Status.SeedName == nil || *shoot.Spec.SeedName == *shoot.Status.SeedName {
			return SeedLabelsMatch(seedLister, *shoot.Spec.SeedName, labelSelector)
		}
		return SeedLabelsMatch(seedLister, *shoot.Status.SeedName, labelSelector)
	}
}

// ShootIsManagedByThisGardenlet checks if the given shoot is managed by this gardenlet by comparing it with the seed name from the GardenletConfiguration
// or by checking whether the seed labels match the seed selector from the GardenletConfiguration.
func ShootIsManagedByThisGardenlet(shoot *gardencorev1beta1.Shoot, gc *config.GardenletConfiguration, seedLister gardencorelisters.SeedLister) bool {
	seedName := confighelper.SeedNameFromSeedConfig(gc.SeedConfig)
	if len(seedName) > 0 {
		return *shoot.Spec.SeedName == seedName
	}
	return SeedLabelsMatch(seedLister, *shoot.Spec.SeedName, gc.SeedSelector)
}

// SeedLabelsMatch fetches the given seed via a lister by its name and then checks whether the given label selector matches
// the seed labels.
func SeedLabelsMatch(seedLister gardencorelisters.SeedLister, seedName string, labelSelector *metav1.LabelSelector) bool {
	seed, err := seedLister.Get(seedName)
	if err != nil {
		return false
	}

	return LabelsMatchFor(seed.Labels, labelSelector)
}

// seedLabelsMatchWithClient fetches the given seed by its name from the client and then checks whether the given
// label selector matches the seed labels.
func seedLabelsMatchWithClient(ctx context.Context, c client.Client, seedName string, labelSelector *metav1.LabelSelector) bool {
	seed := &gardencorev1beta1.Seed{}
	if err := c.Get(ctx, client.ObjectKey{Name: seedName}, seed); err != nil {
		return false
	}

	return LabelsMatchFor(seed.Labels, labelSelector)
}

// ControllerInstallationFilterFunc returns a filtering func for the seeds and the given label selector.
func ControllerInstallationFilterFunc(seedName string, seedLister gardencorelisters.SeedLister, labelSelector *metav1.LabelSelector) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
		if !ok {
			return false
		}
		if len(seedName) > 0 {
			return controllerInstallation.Spec.SeedRef.Name == seedName
		}
		return SeedLabelsMatch(seedLister, controllerInstallation.Spec.SeedRef.Name, labelSelector)
	}
}

// BackupBucketFilterFunc returns a filtering func for the seeds and the given label selector.
func BackupBucketFilterFunc(ctx context.Context, c client.Client, seedName string, labelSelector *metav1.LabelSelector) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
		if !ok {
			return false
		}
		if backupBucket.Spec.SeedName == nil {
			return false
		}
		if len(seedName) > 0 {
			return *backupBucket.Spec.SeedName == seedName
		}
		return seedLabelsMatchWithClient(ctx, c, *backupBucket.Spec.SeedName, labelSelector)
	}
}

// BackupEntryFilterFunc returns a filtering func for the seeds and the given label selector.
func BackupEntryFilterFunc(ctx context.Context, c client.Client, seedName string, labelSelector *metav1.LabelSelector) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
		if !ok {
			return false
		}
		if backupEntry.Spec.SeedName == nil {
			return false
		}
		if len(seedName) > 0 {
			if backupEntry.Status.SeedName == nil || *backupEntry.Spec.SeedName == *backupEntry.Status.SeedName {
				return *backupEntry.Spec.SeedName == seedName
			}
			return *backupEntry.Status.SeedName == seedName
		}
		if backupEntry.Status.SeedName == nil || *backupEntry.Spec.SeedName == *backupEntry.Status.SeedName {
			return seedLabelsMatchWithClient(ctx, c, *backupEntry.Spec.SeedName, labelSelector)
		}
		return seedLabelsMatchWithClient(ctx, c, *backupEntry.Status.SeedName, labelSelector)
	}
}

// BackupEntryIsManagedByThisGardenlet checks if the given BackupEntry is managed by this gardenlet by comparing it with the seed name from the GardenletConfiguration
// or by checking whether the seed labels match the seed selector from the GardenletConfiguration.
func BackupEntryIsManagedByThisGardenlet(ctx context.Context, c client.Client, backupEntry *gardencorev1beta1.BackupEntry, gc *config.GardenletConfiguration) bool {
	seedName := confighelper.SeedNameFromSeedConfig(gc.SeedConfig)
	if len(seedName) > 0 {
		return backupEntry.Spec.SeedName != nil && *backupEntry.Spec.SeedName == seedName
	}
	return seedLabelsMatchWithClient(ctx, c, *backupEntry.Spec.SeedName, gc.SeedSelector)
}

// BastionFilterFunc returns a filtering func for the seeds and the given label selector.
func BastionFilterFunc(ctx context.Context, c client.Client, seedName string, labelSelector *metav1.LabelSelector) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		bastion, ok := obj.(*gardenoperationsv1alpha1.Bastion)
		if !ok {
			return false
		}
		if bastion.Spec.SeedName == nil {
			return false
		}
		if len(seedName) > 0 {
			return *bastion.Spec.SeedName == seedName
		}
		return seedLabelsMatchWithClient(ctx, c, *bastion.Spec.SeedName, labelSelector)
	}
}

// ManagedSeedFilterFunc returns a filtering func for ManagedSeeds that checks if the ManagedSeed references a Shoot scheduled on a Seed, for which the gardenlet is responsible..
func ManagedSeedFilterFunc(ctx context.Context, c client.Client, seedName string, labelSelector *metav1.LabelSelector) func(obj interface{}) bool {
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
		if len(seedName) > 0 {
			if shoot.Status.SeedName == nil || *shoot.Spec.SeedName == *shoot.Status.SeedName {
				return *shoot.Spec.SeedName == seedName
			}
			return *shoot.Status.SeedName == seedName
		}
		if shoot.Status.SeedName == nil || *shoot.Spec.SeedName == *shoot.Status.SeedName {
			return seedLabelsMatchWithClient(ctx, c, *shoot.Spec.SeedName, labelSelector)
		}
		return seedLabelsMatchWithClient(ctx, c, *shoot.Status.SeedName, labelSelector)
	}
}
