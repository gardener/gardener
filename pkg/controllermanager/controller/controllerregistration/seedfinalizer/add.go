// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seedfinalizer

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	"github.com/go-logr/logr"
)

// ControllerName is the name of this controller.
const ControllerName = "controllerregistration-seed-finalizer"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&gardencorev1beta1.Seed{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 5,
		}).
		Build(r)
	if err != nil {
		return err
	}

	return c.Watch(source.Kind[client.Object](mgr.GetCache(),
		&gardencorev1beta1.ControllerInstallation{},
		mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper.MapFunc(r.MapControllerInstallationToSeed), mapper.UpdateWithNew, c.GetLogger()),
		r.ControllerUninstallationPredicate(),
	))
}

// MapControllerInstallationToSeed returns a reconcile.Request object for the seed specified in the .spec.seedRef.name field.
func (r *Reconciler) MapControllerInstallationToSeed(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
	if !ok {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: controllerInstallation.Spec.SeedRef.Name}}}
}

// ControllerUninstallationPredicate returns true for all ControllerInstallation 'delete' events. For other events, false is returned.
func (r *Reconciler) ControllerUninstallationPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(_ event.CreateEvent) bool { return false },
		UpdateFunc:  func(_ event.UpdateEvent) bool { return false },
		DeleteFunc:  func(_ event.DeleteEvent) bool { return true },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}
