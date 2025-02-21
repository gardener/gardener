// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package finalizer

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const ControllerName = "shootstate-finalizer"

func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.client == nil {
		r.client = mgr.GetClient()
	}

	return builder.ControllerManagedBy(mgr).
		Named(ControllerName).
		For(
			&gardencorev1beta1.ShootState{},
			builder.WithPredicates(r.ShootStatePredicates()),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Watches(
			&gardencorev1beta1.Shoot{},
			handler.EnqueueRequestsFromMapFunc(r.MapShootToShootState),
			builder.WithPredicates(r.ShootPredicates()),
		).Complete(r)
}

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

func (r *Reconciler) ShootStatePredicates() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool { return true },
		UpdateFunc: func(e event.UpdateEvent) bool { return true },
	}
}

func (r *Reconciler) ShootPredicates() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool { return true },
	}
}
