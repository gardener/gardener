// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucketscheck

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// ControllerName is the name of this controller.
const ControllerName = "seed-backupbuckets-check"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
		}).
		Watches(
			&gardencorev1beta1.BackupBucket{},
			handler.EnqueueRequestsFromMapFunc(r.MapBackupBucketToSeed),
			builder.WithPredicates(r.BackupBucketPredicate()),
		).
		Complete(r)
}

// BackupBucketPredicate reacts only on 'CREATE' and 'UPDATE' events. It returns false if .spec.seedName == nil. For
// updates, it only returns true when the .status.lastError changed.
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

			if backupBucket.Spec.SeedName == nil {
				return false
			}
			return lastErrorChanged(oldBackupBucket.Status.LastError, backupBucket.Status.LastError)
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

func lastErrorChanged(oldLastError, newLastError *gardencorev1beta1.LastError) bool {
	return (oldLastError == nil && newLastError != nil) ||
		(oldLastError != nil && newLastError == nil) ||
		(oldLastError != nil && newLastError != nil && oldLastError.Description != newLastError.Description)
}

// MapBackupBucketToSeed is a handler.MapFunc for mapping a BackupBucket to the referenced Seed.
func (r *Reconciler) MapBackupBucketToSeed(_ context.Context, obj client.Object) []reconcile.Request {
	backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
	if !ok {
		return nil
	}

	if backupBucket.Spec.SeedName == nil {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: *backupBucket.Spec.SeedName}}}
}
