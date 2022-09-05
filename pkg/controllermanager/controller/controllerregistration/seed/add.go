// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"

	"github.com/go-logr/logr"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ControllerName is the name of this controller.
const ControllerName = "controllerregistration-seed"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.APIReader == nil {
		r.APIReader = mgr.GetAPIReader()
	}

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&gardencorev1beta1.Seed{}, builder.WithPredicates(r.SeedPredicate())).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.ConcurrentSyncs, 0),
			RecoverPanic:            true,
		}).
		Build(r)
	if err != nil {
		return err
	}

	if err := c.Watch(
		&source.Kind{Type: &gardencorev1beta1.ControllerRegistration{}},
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapToAllSeeds), mapper.UpdateWithNew, c.GetLogger()),
		r.ControllerRegistrationPredicate(),
	); err != nil {
		return err
	}

	if err := c.Watch(
		&source.Kind{Type: &gardencorev1beta1.BackupBucket{}},
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapBackupBucketToSeed), mapper.UpdateWithNew, c.GetLogger()),
		r.BackupBucketPredicate(),
	); err != nil {
		return err
	}

	if err := c.Watch(
		&source.Kind{Type: &gardencorev1beta1.BackupEntry{}},
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapBackupEntryToSeed), mapper.UpdateWithNew, c.GetLogger()),
		r.BackupEntryPredicate(),
	); err != nil {
		return err
	}

	return c.Watch(
		&source.Kind{Type: &gardencorev1beta1.Shoot{}},
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapShootToSeed), mapper.UpdateWithNew, c.GetLogger()),
		r.ShootPredicate(),
	)
}

// SeedPredicate returns true for all Seed events except for updates. Here, it only returns true when there is a change
// in the .spec.dns.provider field or when the deletion timestamp is set.
func (r *Reconciler) SeedPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			seed, ok := e.ObjectNew.(*gardencorev1beta1.Seed)
			if !ok {
				return false
			}

			oldSeed, ok := e.ObjectOld.(*gardencorev1beta1.Seed)
			if !ok {
				return false
			}

			return !apiequality.Semantic.DeepEqual(oldSeed.Spec.DNS.Provider, seed.Spec.DNS.Provider) ||
				seed.DeletionTimestamp != nil
		},
	}
}

// ControllerRegistrationPredicate returns true for all create and update events.
func (r *Reconciler) ControllerRegistrationPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		UpdateFunc:  func(e event.UpdateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
}

// BackupBucketPredicate returns true for all BackupBucket events when there is a non-nil .spec.seedName. For updates,
// it only returns true when there is a change in the .spec.seedName or .spec.provider.type fields.
func (r *Reconciler) BackupBucketPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			backupBucket, ok := e.Object.(*gardencorev1beta1.BackupBucket)
			if !ok {
				return false
			}
			return backupBucket.Spec.SeedName != nil
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

			return !apiequality.Semantic.DeepEqual(oldBackupBucket.Spec.SeedName, backupBucket.Spec.SeedName) ||
				oldBackupBucket.Spec.Provider.Type != backupBucket.Spec.Provider.Type
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			backupBucket, ok := e.Object.(*gardencorev1beta1.BackupBucket)
			if !ok {
				return false
			}
			return backupBucket.Spec.SeedName != nil
		},
	}
}

// BackupEntryPredicate returns true for all BackupEntry events when there is a non-nil .spec.seedName. For updates,
// it only returns true when there is a change in the .spec.seedName or .spec.bucketName fields.
func (r *Reconciler) BackupEntryPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			backupEntry, ok := e.Object.(*gardencorev1beta1.BackupEntry)
			if !ok {
				return false
			}
			return backupEntry.Spec.SeedName != nil
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

			return !apiequality.Semantic.DeepEqual(oldBackupEntry.Spec.SeedName, backupEntry.Spec.SeedName) ||
				oldBackupEntry.Spec.BucketName != backupEntry.Spec.BucketName
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			backupEntry, ok := e.Object.(*gardencorev1beta1.BackupEntry)
			if !ok {
				return false
			}
			return backupEntry.Spec.SeedName != nil
		},
	}
}

// ShootPredicate returns true for all Shoot events when there is a non-nil .spec.seedName. For updates, it only returns
// true when there is a change in the .spec.seedName or .spec.provider.workers or .spec.extensions or .spec.dns or
// .spec.networking.type or .spec.provider.type fields.
func (r *Reconciler) ShootPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			shoot, ok := e.Object.(*gardencorev1beta1.Shoot)
			if !ok {
				return false
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

			return !apiequality.Semantic.DeepEqual(oldShoot.Spec.SeedName, shoot.Spec.SeedName) ||
				!apiequality.Semantic.DeepEqual(oldShoot.Spec.Provider.Workers, shoot.Spec.Provider.Workers) ||
				!apiequality.Semantic.DeepEqual(oldShoot.Spec.Extensions, shoot.Spec.Extensions) ||
				!apiequality.Semantic.DeepEqual(oldShoot.Spec.DNS, shoot.Spec.DNS) ||
				oldShoot.Spec.Networking.Type != shoot.Spec.Networking.Type ||
				oldShoot.Spec.Provider.Type != shoot.Spec.Provider.Type
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			shoot, ok := e.Object.(*gardencorev1beta1.Shoot)
			if !ok {
				return false
			}
			return shoot.Spec.SeedName != nil
		},
	}
}

// MapToAllSeeds returns reconcile.Request objects for all existing seeds in the system.
func (r *Reconciler) MapToAllSeeds(ctx context.Context, log logr.Logger, reader client.Reader, _ client.Object) []reconcile.Request {
	seedList := &metav1.PartialObjectMetadataList{}
	seedList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("SeedList"))
	if err := reader.List(ctx, seedList); err != nil {
		log.Error(err, "Failed to list seeds")
		return nil
	}

	return mapper.ObjectListToRequests(seedList)
}

// MapBackupBucketToSeed returns a reconcile.Request object for the seed specified in the .spec.seedName field.
func (r *Reconciler) MapBackupBucketToSeed(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
	if !ok {
		return nil
	}

	if backupBucket.Spec.SeedName == nil {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: *backupBucket.Spec.SeedName}}}
}

// MapBackupEntryToSeed returns a reconcile.Request object for the seed specified in the .spec.seedName field.
func (r *Reconciler) MapBackupEntryToSeed(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
	if !ok {
		return nil
	}

	if backupEntry.Spec.SeedName == nil {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: *backupEntry.Spec.SeedName}}}
}

// MapShootToSeed returns a reconcile.Request object for the seed specified in the .spec.seedName field.
func (r *Reconciler) MapShootToSeed(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	shoot, ok := obj.(*gardencorev1beta1.Shoot)
	if !ok {
		return nil
	}

	if shoot.Spec.SeedName == nil {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: *shoot.Spec.SeedName}}}
}
