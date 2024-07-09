//  SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
//  SPDX-License-Identifier: Apache-2.0

package predicate

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// GardenPredicate is a predicate which returns 'true' for create events, and for update events in case the garden was
// successfully reconciled.
func GardenPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			garden, ok := e.ObjectNew.(*operatorv1alpha1.Garden)
			if !ok {
				return false
			}

			oldGarden, ok := e.ObjectOld.(*operatorv1alpha1.Garden)
			if !ok {
				return false
			}

			// re-evaluate health status right after a reconciliation operation has succeeded
			return predicateutils.ReconciliationFinishedSuccessfully(oldGarden.Status.LastOperation, garden.Status.LastOperation)
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

// DeletePredicate is a predicate that is true only in the event of an object deletion.
func DeletePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return false },
		UpdateFunc:  func(event.UpdateEvent) bool { return false },
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}
