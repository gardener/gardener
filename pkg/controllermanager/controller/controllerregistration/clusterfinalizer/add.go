// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clusterfinalizer

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// ControllerName is the name of this controller.
const ControllerName = "controllerregistration-cluster-finalizer"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, controllerInstallationEventHandler handler.EventHandler, objKind string) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName+"-"+objKind).
		For(r.NewTargetObjectFunc(), builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 5,
			ReconciliationTimeout:   controllerutils.DefaultReconciliationTimeout,
		}).
		Watches(
			&gardencorev1beta1.ControllerInstallation{},
			controllerInstallationEventHandler,
		).
		Complete(r)
}

// ControllerInstallationEventHandlerForSeed returns an event handler that enqueues the seed referenced by a
// ControllerInstallation on delete and update events. On updates, the old object's `.spec.seedRef` is used because the
// seed reconciler clears `.spec.seedRef` when a seed no longer needs a ControllerInstallation.
func ControllerInstallationEventHandlerForSeed() handler.EventHandler {
	return &handler.Funcs{
		DeleteFunc: func(_ context.Context, evt event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			controllerInstallation, ok := evt.Object.(*gardencorev1beta1.ControllerInstallation)
			if !ok || controllerInstallation.Spec.SeedRef == nil {
				return
			}

			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerInstallation.Spec.SeedRef.Name}})
		},
		UpdateFunc: func(_ context.Context, evt event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			oldControllerInstallation, ok := evt.ObjectOld.(*gardencorev1beta1.ControllerInstallation)
			if !ok || oldControllerInstallation.Spec.SeedRef == nil {
				return
			}

			newControllerInstallation, ok := evt.ObjectNew.(*gardencorev1beta1.ControllerInstallation)
			if !ok {
				return
			}

			// Enqueue the seed when `.spec.seedRef` was cleared.
			if newControllerInstallation.Spec.SeedRef == nil {
				q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: oldControllerInstallation.Spec.SeedRef.Name}})
			}
		},
	}
}

// ControllerInstallationEventHandlerForShoot returns an event handler that enqueues the shoot referenced by a
// ControllerInstallation on delete events.
func ControllerInstallationEventHandlerForShoot() handler.EventHandler {
	return &handler.Funcs{
		DeleteFunc: func(_ context.Context, evt event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			controllerInstallation, ok := evt.Object.(*gardencorev1beta1.ControllerInstallation)
			if !ok || controllerInstallation.Spec.ShootRef == nil {
				return
			}

			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerInstallation.Spec.ShootRef.Name, Namespace: controllerInstallation.Spec.ShootRef.Namespace}})
		},
	}
}
