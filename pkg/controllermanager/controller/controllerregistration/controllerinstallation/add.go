// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerinstallation

import (
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// BackupBucketPredicate returns true for all BackupBucket events. For updates, it only returns true when there is a
// change in the .spec.seedName or .spec.provider.type fields.
func BackupBucketPredicate(kind Kind) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			backupBucket, ok := e.Object.(*gardencorev1beta1.BackupBucket)
			if !ok {
				return false
			}

			return (kind == ShootKind) == (backupBucket.Spec.SeedName == nil)
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			backupBucket, ok := e.ObjectNew.(*gardencorev1beta1.BackupBucket)
			if !ok {
				return false
			}

			oldBackupBucket, ok := e.ObjectOld.(*gardencorev1beta1.BackupBucket)
			if !ok {
				return false
			}

			if (kind == ShootKind) && backupBucket.Spec.SeedName != nil {
				return false
			}

			return !apiequality.Semantic.DeepEqual(oldBackupBucket.Spec.SeedName, backupBucket.Spec.SeedName) ||
				oldBackupBucket.Spec.Provider.Type != backupBucket.Spec.Provider.Type
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			backupBucket, ok := e.Object.(*gardencorev1beta1.BackupBucket)
			if !ok {
				return false
			}
			return (kind == ShootKind) == (backupBucket.Spec.SeedName == nil)
		},
	}
}

// BackupEntryPredicate returns true for all BackupEntry events. For updates, it only returns true when there is a
// change in the .spec.seedName or .spec.bucketName fields.
func BackupEntryPredicate(kind Kind) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			backupEntry, ok := e.Object.(*gardencorev1beta1.BackupEntry)
			if !ok {
				return false
			}
			return (kind == ShootKind) == (backupEntry.Spec.SeedName == nil)
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			backupEntry, ok := e.ObjectNew.(*gardencorev1beta1.BackupEntry)
			if !ok {
				return false
			}

			oldBackupEntry, ok := e.ObjectOld.(*gardencorev1beta1.BackupEntry)
			if !ok {
				return false
			}

			if (kind == ShootKind) && backupEntry.Spec.SeedName != nil {
				return false
			}

			return !apiequality.Semantic.DeepEqual(oldBackupEntry.Spec.SeedName, backupEntry.Spec.SeedName) ||
				oldBackupEntry.Spec.BucketName != backupEntry.Spec.BucketName
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			backupEntry, ok := e.Object.(*gardencorev1beta1.BackupEntry)
			if !ok {
				return false
			}
			return (kind == ShootKind) == (backupEntry.Spec.SeedName == nil)
		},
	}
}

// ControllerInstallationPredicate returns true for all ControllerInstallation 'create' events. For updates, it only
// returns true when the Required condition's status has changed. For other events, false is returned.
func ControllerInstallationPredicate(kind Kind) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if kind == SeedKind {
				return true
			}

			controllerInstallation, ok := e.Object.(*gardencorev1beta1.ControllerInstallation)
			if !ok {
				return false
			}

			return controllerInstallation.Spec.ShootRef != nil
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			controllerInstallation, ok := e.ObjectNew.(*gardencorev1beta1.ControllerInstallation)
			if !ok {
				return false
			}

			oldControllerInstallation, ok := e.ObjectOld.(*gardencorev1beta1.ControllerInstallation)
			if !ok {
				return false
			}

			if kind == ShootKind && controllerInstallation.Spec.SeedRef != nil {
				return false
			}

			return v1beta1helper.IsControllerInstallationRequired(*oldControllerInstallation) != v1beta1helper.IsControllerInstallationRequired(*controllerInstallation)
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

// ShootPredicate returns true for all Shoot events. For updates, it only returns true when there is a change in the
// .spec.provider.workers or .spec.extensions or .spec.dns or .spec.networking.type or .spec.provider.type fields.
func ShootPredicate(kind Kind) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			shoot, ok := e.Object.(*gardencorev1beta1.Shoot)
			if !ok {
				return false
			}

			if kind == ShootKind {
				return v1beta1helper.IsShootSelfHosted(shoot.Spec.Provider.Workers)
			}

			return shoot.Spec.SeedName != nil
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			shoot, ok := e.ObjectNew.(*gardencorev1beta1.Shoot)
			if !ok {
				return false
			}

			oldShoot, ok := e.ObjectOld.(*gardencorev1beta1.Shoot)
			if !ok {
				return false
			}

			if kind == ShootKind && !v1beta1helper.IsShootSelfHosted(shoot.Spec.Provider.Workers) {
				return false
			}

			return (kind == SeedKind && !apiequality.Semantic.DeepEqual(oldShoot.Spec.SeedName, shoot.Spec.SeedName)) ||
				!apiequality.Semantic.DeepEqual(oldShoot.Spec.Provider.Workers, shoot.Spec.Provider.Workers) ||
				!apiequality.Semantic.DeepEqual(oldShoot.Spec.Extensions, shoot.Spec.Extensions) ||
				!apiequality.Semantic.DeepEqual(oldShoot.Spec.DNS, shoot.Spec.DNS) ||
				shootNetworkingTypeHasChanged(oldShoot.Spec.Networking, shoot.Spec.Networking) ||
				oldShoot.Spec.Provider.Type != shoot.Spec.Provider.Type ||
				(kind == ShootKind && shoot.DeletionTimestamp != nil)
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			shoot, ok := e.Object.(*gardencorev1beta1.Shoot)
			if !ok {
				return false
			}

			if kind == ShootKind {
				return v1beta1helper.IsShootSelfHosted(shoot.Spec.Provider.Workers)
			}

			return shoot.Spec.SeedName != nil
		},
	}
}

func shootNetworkingTypeHasChanged(old, new *gardencorev1beta1.Networking) bool {
	if old == nil && new == nil {
		return false
	}
	if old == nil && new != nil {
		// if new is non-nil then return true if new has a type set
		return new.Type != nil
	}
	if old != nil && new == nil {
		// if old was non-nil and had a type set, return true
		return old.Type != nil
	}
	return !ptr.Equal(old.Type, new.Type)
}
