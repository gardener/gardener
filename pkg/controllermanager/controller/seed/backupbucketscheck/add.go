// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucketscheck

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
)

// ControllerName is the name of this controller.
const ControllerName = "seed-backupbuckets-check"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	// It's not possible to call builder.Build() without adding atleast one watch, and without this, we can't get the controller logger.
	// Hence, we have to build up the controller manually.
	c, err := controller.New(
		ControllerName,
		mgr,
		controller.Options{
			Reconciler:              r,
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
			// if going into exponential backoff, wait at most the configured sync period
			RateLimiter: workqueue.NewTypedWithMaxWaitRateLimiter(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](), r.Config.SyncPeriod.Duration),
		},
	)
	if err != nil {
		return err
	}

	return c.Watch(
		source.Kind[client.Object](mgr.GetCache(),
			&gardencorev1beta1.BackupBucket{},
			mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper.MapFunc(r.MapBackupBucketToSeed), mapper.UpdateWithNew, c.GetLogger()),
			r.BackupBucketPredicate(),
		))
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

// MapBackupBucketToSeed is a mapper.MapFunc for mapping a BackupBucket to the referenced Seed.
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
