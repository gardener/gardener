// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package predicate

import (
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// DefaultControllerPredicates returns the default predicates for extension controllers. If the operation annotation
// is ignored then the only returned predicate is the 'GenerationChangedPredicate'.
func DefaultControllerPredicates(ignoreOperationAnnotation bool, preconditions ...predicate.Predicate) []predicate.Predicate {
	if ignoreOperationAnnotation {
		return append(preconditions, predicate.GenerationChangedPredicate{})
	}
	return append(preconditions, defaultControllerPredicate)
}

var defaultControllerPredicate = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		if e.Object == nil {
			return false
		}

		// If a relevant operation annotation is present then we admit reconciliation.
		if hasOperationAnnotation(e.Object) {
			return true
		}

		// If the object's deletion timestamp is set then we admit reconciliation.
		// Note that while an object cannot be created with a deletion timestamp, on startup, the controller receives
		// ADD/CREATE events for all existing objects which might already have a deletion timestamp.
		if e.Object.GetDeletionTimestamp() != nil {
			return true
		}

		// If the last operation does not indicate success then we admit reconciliation. This also means triggers if the
		// last operation is not yet set.
		// Note that this check is only performed for CREATE events in order to trigger a retry after controller restart.
		// If it also reacted on UPDATE events the controller would constantly enqueue the resource when other updates
		// (like status changes) happen, i.e., it might trigger itself endlessly when a previous reconciliation failed
		// (since it updated the last operation to 'error').
		if lastOperationNotSuccessful(e.Object) {
			return true
		}

		// If none of the above conditions applies then reconciliation is not allowed.
		return false
	},

	UpdateFunc: func(e event.UpdateEvent) bool {
		// If a relevant operation annotation is present then we admit reconciliation. The OperationAnnotationWrapper
		// ensures that this annotation is removed right after the reconciler has picked up the object. This prevents
		// that both the OperationAnnotationWrapper and the reconciler endlessly trigger further events when removing
		// the annotation or updating the status.
		if hasOperationAnnotation(e.ObjectNew) {
			return true
		}

		// If the object's deletion timestamp is set and the status has not changed then we admit reconciliation. This
		// covers the actual delete request (which once increases the generation) and further updates (e.g., when
		// gardenlet updates the timestamp annotation). It prevents that the controller triggers itself endlessly when
		// updating the status while the deletion timestamp is set.
		if e.ObjectNew.GetDeletionTimestamp() != nil && statusEqual(e.ObjectOld, e.ObjectNew) {
			return true
		}

		// If none of the above conditions applies then reconciliation is not allowed.
		return false
	},

	DeleteFunc:  func(event.DeleteEvent) bool { return false },
	GenericFunc: func(event.GenericEvent) bool { return false },
}

func hasOperationAnnotation(obj client.Object) bool {
	return obj.GetAnnotations()[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationReconcile ||
		obj.GetAnnotations()[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore ||
		obj.GetAnnotations()[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationMigrate
}

func lastOperationNotSuccessful(obj client.Object) bool {
	acc, err := extensions.Accessor(obj)
	if err != nil {
		return false // Error is ignored here since we cannot do anything meaningful with it.
	}

	lastOp := acc.GetExtensionStatus().GetLastOperation()
	return lastOp == nil || lastOp.State != gardencorev1beta1.LastOperationStateSucceeded
}

func statusEqual(oldObj, newObj client.Object) bool {
	oldAcc, err1 := extensions.Accessor(oldObj)
	newAcc, err2 := extensions.Accessor(newObj)
	if err1 != nil || err2 != nil {
		return false // Errors are ignored here since we cannot do anything meaningful with them.
	}

	return apiequality.Semantic.DeepEqual(oldAcc.GetExtensionStatus(), newAcc.GetExtensionStatus())
}
