// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootstate

import (
	"context"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	logger := mgr.GetLogger().WithValues("controller", ControllerName)
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
			handler.EnqueueRequestsFromMapFunc(r.MapShootToShootState(logger)),
			builder.WithPredicates(r.ShootPredicates()),
		).Complete(r)
}

// MapShootToShootState maps a Shoot object to ShootState reconciliation request.
func (r *Reconciler) MapShootToShootState(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		shoot, ok := obj.(*gardencorev1beta1.Shoot)
		if !ok {
			return nil
		}

		shootState := &gardencorev1beta1.ShootState{}
		if err := r.Client.Get(ctx, client.ObjectKeyFromObject(shoot), shootState); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error(err, "Failed to get ShootState for Shoot", "shoot", client.ObjectKeyFromObject(shoot))
			}
			return nil
		}

		namespacedName := types.NamespacedName{
			Name:      shoot.Name,
			Namespace: shoot.Namespace,
		}
		return []reconcile.Request{{NamespacedName: namespacedName}}
	}
}

// ShootPredicates returns predicates for Shoot requests acceptance.
func (r *Reconciler) ShootPredicates() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return false },
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
				oldLastOpState = shootOld.Status.LastOperation.State
				newLastOpState = shoot.Status.LastOperation.State

				isOldLastOpMigrate   = oldLastOpType == gardencorev1beta1.LastOperationTypeMigrate
				isOldLastOpRestore   = oldLastOpType == gardencorev1beta1.LastOperationTypeRestore
				isOldLastOpSucceeded = oldLastOpState == gardencorev1beta1.LastOperationStateSucceeded

				isNewLastOpReconcile = newLastOpType == gardencorev1beta1.LastOperationTypeReconcile
				isNewLastOpMigrate   = newLastOpType == gardencorev1beta1.LastOperationTypeMigrate
				isNewLastOpRestore   = newLastOpType == gardencorev1beta1.LastOperationTypeRestore
				isNewLastOpSucceeded = newLastOpState == gardencorev1beta1.LastOperationStateSucceeded
			)

			enqueueShootState := false
			if !isOldLastOpMigrate && isNewLastOpMigrate {
				// Shoot last operation gets updated from non-Migrate to Migrate type.
				enqueueShootState = true
			} else if isOldLastOpMigrate && isNewLastOpRestore {
				// Shoot last operation gets updated from Migrate to Restore type.
				enqueueShootState = true
			} else if isOldLastOpRestore && !isOldLastOpSucceeded && isNewLastOpRestore && isNewLastOpSucceeded {
				// Shoot last operation gets updated from Restore to Restore type with Succeeded state.
				enqueueShootState = true
			} else if isOldLastOpRestore && !isOldLastOpSucceeded && isNewLastOpReconcile {
				// Shoot last operation gets updated from Restore to Reconcile.
				enqueueShootState = true
			}
			return enqueueShootState
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}
