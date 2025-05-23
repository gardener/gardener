// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package predicate

import (
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

var classChangedPredicate = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		if e.ObjectOld == nil {
			return false
		}
		if e.ObjectNew == nil {
			return false
		}

		oldObj, ok := e.ObjectOld.(*resourcesv1alpha1.ManagedResource)
		if !ok {
			return false
		}
		newObj, ok := e.ObjectNew.(*resourcesv1alpha1.ManagedResource)
		if !ok {
			return false
		}

		return !equality.Semantic.DeepEqual(oldObj.Spec.Class, newObj.Spec.Class)
	},
}

// ClassChangedPredicate is a predicate for changes in `.spec.class`.
func ClassChangedPredicate() predicate.Predicate {
	return classChangedPredicate
}
