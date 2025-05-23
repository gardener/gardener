// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package predicate

import (
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

// NotIgnored returns a predicate that detects if the object has the resources.gardener.cloud/ignore=true annotation.
func NotIgnored() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return !isIgnored(obj)
	})
}

// NoLongerIgnored returns a predicate that detects if resources.gardener.cloud/ignore=true annotation was removed
// during an update.
func NoLongerIgnored() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return isIgnored(e.ObjectOld) && !isIgnored(e.ObjectNew)
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return true
		},
	}
}

// GotMarkedAsIgnored returns a predicate that detects if resources.gardener.cloud/ignore=true annotation was added
// during an update.
func GotMarkedAsIgnored() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool {
			// Add event is received on controller startup. We need to reconcile this if we haven't updated the conditions yet
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return !isIgnored(e.ObjectOld) && isIgnored(e.ObjectNew)
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return true
		},
	}
}

func isIgnored(obj client.Object) bool {
	value, ok := obj.GetAnnotations()[resourcesv1alpha1.Ignore]
	if !ok {
		return false
	}
	truthy, _ := strconv.ParseBool(value)
	return truthy
}
