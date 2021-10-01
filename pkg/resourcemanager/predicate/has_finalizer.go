// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
