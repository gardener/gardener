// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensions

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// ObjectPredicate returns true for 'create' and 'delete' events. For updates, it only returns true when the extension
// type has changed.
func ObjectPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			// enqueue on periodic cache resyncs
			if e.ObjectOld.GetResourceVersion() == e.ObjectNew.GetResourceVersion() {
				return true
			}

			extensionObj, ok := e.ObjectNew.(extensionsv1alpha1.Object)
			if !ok {
				return false
			}

			oldExtensionObj, ok := e.ObjectOld.(extensionsv1alpha1.Object)
			if !ok {
				return false
			}

			return oldExtensionObj.GetExtensionSpec().GetExtensionType() != extensionObj.GetExtensionSpec().GetExtensionType()
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return false
		},
	}
}
