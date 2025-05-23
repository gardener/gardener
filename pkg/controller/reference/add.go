// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reference

import (
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// controllerNameSuffix is the suffix added to the controller name.
const controllerNameSuffix = "-reference"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, name string) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(name+controllerNameSuffix).
		For(r.NewObjectFunc(), builder.WithPredicates(r.Predicate())).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.ConcurrentSyncs, 0),
		}).
		Complete(r)
}

// Predicate reacts on CREATE and on UPDATE events.
func (r *Reconciler) Predicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return r.ReferenceChangedPredicate(e.ObjectOld, e.ObjectNew) ||
				(e.ObjectNew.GetDeletionTimestamp() != nil && !controllerutil.ContainsFinalizer(e.ObjectNew, gardencorev1beta1.GardenerName))
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}
