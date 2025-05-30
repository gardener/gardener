//  SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
//  SPDX-License-Identifier: Apache-2.0

package predicate

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// GardenCreatedOrReconciledSuccessfully is a predicate which returns 'true' for create events, and for update events in case the garden was
// successfully reconciled.
func GardenCreatedOrReconciledSuccessfully() predicate.Predicate {
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

			return predicateutils.ReconciliationFinishedSuccessfully(oldGarden.Status.LastOperation, garden.Status.LastOperation)
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

// GardenDeletionTriggered is a predicate which returns 'true' for update events,
// if the deletion of the Garden resource was initiated.
func GardenDeletionTriggered() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectNew.GetDeletionTimestamp() != nil && e.ObjectOld.GetDeletionTimestamp() == nil
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}
