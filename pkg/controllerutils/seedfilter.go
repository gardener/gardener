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
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
			return *shoot.Spec.SeedName == seedName
		}
		return seedLabelsMatch(seedLister, *shoot.Spec.SeedName, labelSelector)
	}
}

func seedLabelsMatch(seedLister gardencorelisters.SeedLister, seedName string, labelSelector *metav1.LabelSelector) bool {
	seed, err := seedLister.Get(seedName)
	if err != nil {
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
		return seedLabelsMatch(seedLister, controllerInstallation.Spec.SeedRef.Name, labelSelector)
	}
}

// BackupBucketFilterFunc returns a filtering func for the seeds and the given label selector.
func BackupBucketFilterFunc(seedName string, seedLister gardencorelisters.SeedLister, labelSelector *metav1.LabelSelector) func(obj interface{}) bool {
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
		return seedLabelsMatch(seedLister, *backupBucket.Spec.SeedName, labelSelector)
	}
}

// BackupEntryFilterFunc returns a filtering func for the seeds and the given label selector.
func BackupEntryFilterFunc(seedName string, seedLister gardencorelisters.SeedLister, labelSelector *metav1.LabelSelector) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
		if !ok {
			return false
		}
		if backupEntry.Spec.SeedName == nil {
			return false
		}
		if len(seedName) > 0 {
			return *backupEntry.Spec.SeedName == seedName
		}
		return seedLabelsMatch(seedLister, *backupEntry.Spec.SeedName, labelSelector)
	}
}
