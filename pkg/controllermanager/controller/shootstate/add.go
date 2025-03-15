// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootstate

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
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
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// ControllerName is the name of the controller.
const ControllerName = "shootstate-finalizer"

// AddToManager adds the Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	return builder.ControllerManagedBy(mgr).
		Named(ControllerName).
		For(
			&gardencorev1beta1.ShootState{},
			builder.WithPredicates(predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update)),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
		}).
		Watches(
			&gardencorev1beta1.Shoot{},
			handler.EnqueueRequestsFromMapFunc(r.MapShootToShootState),
			builder.WithPredicates(r.ShootPredicates()),
		).Complete(r)
}

// MapShootToShootState maps a Shoot object to ShootState reconciliation request.
func (r *Reconciler) MapShootToShootState(_ context.Context, obj client.Object) []reconcile.Request {
	shoot, ok := obj.(*gardencorev1beta1.Shoot)
	if !ok {
		return nil
	}

	namespacedName := types.NamespacedName{
		Name:      shoot.Name,
		Namespace: shoot.Namespace,
	}
	return []reconcile.Request{{NamespacedName: namespacedName}}
}

// ShootPredicates returns predicates for Shoot requests acceptance.
func (r *Reconciler) ShootPredicates() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			shoot, ok := e.ObjectNew.(*gardencorev1beta1.Shoot)
			if !ok || shoot.Status.LastOperation == nil {
				return false
			}

			shootOld, ok := e.ObjectOld.(*gardencorev1beta1.Shoot)
			if !ok || shootOld.Status.LastOperation == nil {
				return false
			}

			var (
				oldLastOpType  = shootOld.Status.LastOperation.Type
				newLastOpType  = shoot.Status.LastOperation.Type
				newLastOpState = shoot.Status.LastOperation.State

				isOldLastOpReconcile = oldLastOpType == gardencorev1beta1.LastOperationTypeReconcile
				isOldLastOpMigrate   = oldLastOpType == gardencorev1beta1.LastOperationTypeMigrate
				isOldLastOpRestore   = oldLastOpType == gardencorev1beta1.LastOperationTypeRestore

				isNewLastOpReconcile = newLastOpType == gardencorev1beta1.LastOperationTypeReconcile
				isNewLastOpMigrate   = newLastOpType == gardencorev1beta1.LastOperationTypeMigrate
				isNewLastOpRestore   = newLastOpType == gardencorev1beta1.LastOperationTypeRestore
				isNewLastOpSucceeded = newLastOpState == gardencorev1beta1.LastOperationStateSucceeded
			)

			isMigrating := false
			if isOldLastOpReconcile && isNewLastOpMigrate {
				// Shoot last operation gets updated from Reconcile to Migrate type.
				isMigrating = true
			} else if isOldLastOpMigrate && isNewLastOpRestore {
				// Shoot last operation gets updated from Migrate to Restore type.
				isMigrating = true
			} else if isOldLastOpRestore && isNewLastOpRestore && isNewLastOpSucceeded {
				// Shoot last operation gets updated from Restore to Restore type with Succeeded state.
				isMigrating = true
			} else if isOldLastOpRestore && isNewLastOpReconcile {
				// Shoot last operation gets updated from Restore to Reconcile.
				isMigrating = true
			}

			isShootStatePresent := true
			shootState := &gardencorev1beta1.ShootState{}
			ctx := context.Background()
			if err := r.Client.Get(ctx, client.ObjectKeyFromObject(shoot), shootState); err != nil {
				isShootStatePresent = false
			}
			return isMigrating && isShootStatePresent
		},
	}
}
