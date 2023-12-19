// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package predicate

import (
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// HasFinalizer returns a predicate that detects if the object has the given finalizer
// This is used to not requeue all secrets in the cluster (which might be quite a lot),
// but only requeue secrets from create/update events with the controller's finalizer.
// This is to ensure, that we properly remove the finalizer in case we missed an important
// update event for a ManagedResource.
func HasFinalizer(finalizer string) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// Create event is emitted on start-up, when the cache is populated from a complete list call for the first time.
			// We should enqueue all secrets, which have the controller's finalizer in order to remove it in case we missed
			// an important update event to a ManagedResource during downtime.
			return controllerutil.ContainsFinalizer(e.Object, finalizer)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			// We only need to check MetaNew. If the finalizer was in MetaOld and is not in MetaNew, it is already
			// removed and we don't need to reconcile the secret.
			return controllerutil.ContainsFinalizer(e.ObjectNew, finalizer)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// If the secret is already deleted, all finalizers are already gone and we don't need to reconcile it.
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
