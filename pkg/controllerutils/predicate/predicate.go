// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package predicate

import (
	"reflect"

	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

// IsDeleting is a predicate for objects having a deletion timestamp.
func IsDeleting() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetDeletionTimestamp() != nil
	})
}

// HasName returns a predicate which returns true when the object has the provided name.
func HasName(name string) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == name
	})
}

// EventType is an alias for byte.
type EventType byte

const (
	// Create is a constant for an event of type 'create'.
	Create EventType = iota
	// Update is a constant for an event of type 'update'.
	Update
	// Delete is a constant for an event of type 'delete'.
	Delete
	// Generic is a constant for an event of type 'generic'.
	Generic
)

// ForEventTypes is a predicate which returns true only for the provided event types.
func ForEventTypes(events ...EventType) predicate.Predicate {
	has := func(event EventType) bool {
		for _, e := range events {
			if e == event {
				return true
			}
		}
		return false
	}

	return predicate.Funcs{
		CreateFunc:  func(_ event.CreateEvent) bool { return has(Create) },
		UpdateFunc:  func(_ event.UpdateEvent) bool { return has(Update) },
		DeleteFunc:  func(_ event.DeleteEvent) bool { return has(Delete) },
		GenericFunc: func(_ event.GenericEvent) bool { return has(Generic) },
	}
}

// EvalGeneric returns true if all predicates match for the given object.
func EvalGeneric(obj client.Object, predicates ...predicate.Predicate) bool {
	e := event.GenericEvent{Object: obj}
	for _, p := range predicates {
		if !p.Generic(e) {
			return false
		}
	}

	return true
}

// RelevantConditionsChanged returns true for all events except for 'UPDATE'. Here, true is only returned when the
// status, reason or message of a relevant condition has changed.
func RelevantConditionsChanged(
	getConditionsFromObject func(obj client.Object) []gardencorev1beta1.Condition,
	relevantConditionTypes ...gardencorev1beta1.ConditionType,
) predicate.Predicate {
	wasConditionStatusReasonOrMessageUpdated := func(oldCondition, newCondition *gardencorev1beta1.Condition) bool {
		return (oldCondition == nil && newCondition != nil) ||
			(oldCondition != nil && newCondition == nil) ||
			(oldCondition != nil && newCondition != nil &&
				(oldCondition.Status != newCondition.Status || oldCondition.Reason != newCondition.Reason || oldCondition.Message != newCondition.Message))
	}

	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			var (
				oldConditions = getConditionsFromObject(e.ObjectOld)
				newConditions = getConditionsFromObject(e.ObjectNew)
			)

			for _, condition := range relevantConditionTypes {
				if wasConditionStatusReasonOrMessageUpdated(
					v1beta1helper.GetCondition(oldConditions, condition),
					v1beta1helper.GetCondition(newConditions, condition),
				) {
					return true
				}
			}

			return false
		},
	}
}

// ManagedResourceConditionsChanged returns a predicate which returns true if the status/reason/message of the
// Resources{Applied,Healthy,Progressing} condition of the ManagedResource changes.
func ManagedResourceConditionsChanged() predicate.Predicate {
	return RelevantConditionsChanged(
		func(obj client.Object) []gardencorev1beta1.Condition {
			managedResource, ok := obj.(*resourcesv1alpha1.ManagedResource)
			if !ok {
				return nil
			}
			return managedResource.Status.Conditions
		},
		resourcesv1alpha1.ResourcesApplied,
		resourcesv1alpha1.ResourcesHealthy,
		resourcesv1alpha1.ResourcesProgressing,
	)
}

// LastOperationChanged returns a predicate which returns true when the LastOperation of the passed object is changed.
func LastOperationChanged(getLastOperation func(client.Object) *gardencorev1beta1.LastOperation) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// If the object has the operation annotation reconcile, this means it's not picked up by the extension controller.
			// For restore and migrate operations, we remove the annotation only at the end, so we don't stop enqueueing it.
			if e.Object.GetAnnotations()[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationReconcile {
				return false
			}

			// If lastOperation State is failed then we admit reconciliation.
			// This is not possible during create but possible during a controller restart.
			return lastOperationStateFailed(getLastOperation(e.Object))
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			// If the object has the operation annotation, this means it's not picked up by the extension controller.
			// migrate and restore annotations are removed for the extensions only at the end of the operation,
			// so if the oldObject doesn't have the same annotation, don't enqueue it.
			if v1beta1helper.HasOperationAnnotation(e.ObjectNew.GetAnnotations()) {
				operation := e.ObjectNew.GetAnnotations()[v1beta1constants.GardenerOperation]

				if operation != v1beta1constants.GardenerOperationMigrate && operation != v1beta1constants.GardenerOperationRestore {
					return false
				}

				// if the oldObject doesn't have the same annotation skip
				if e.ObjectOld.GetAnnotations()[v1beta1constants.GardenerOperation] != operation {
					return false
				}
			}

			// If lastOperation State has changed to Succeeded or Error then we admit reconciliation.
			return lastOperationStateChanged(getLastOperation(e.ObjectOld), getLastOperation(e.ObjectNew))
		},

		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

// ReconciliationFinishedSuccessfully is a helper function for checking whether the last operation indicates a
// successful reconciliation.
func ReconciliationFinishedSuccessfully(oldLastOperation, newLastOperation *gardencorev1beta1.LastOperation) bool {
	return oldLastOperation != nil &&
		oldLastOperation.Type != gardencorev1beta1.LastOperationTypeDelete &&
		oldLastOperation.State == gardencorev1beta1.LastOperationStateProcessing &&
		newLastOperation != nil &&
		newLastOperation.Type != gardencorev1beta1.LastOperationTypeDelete &&
		newLastOperation.State == gardencorev1beta1.LastOperationStateSucceeded
}

func lastOperationStateFailed(lastOperation *gardencorev1beta1.LastOperation) bool {
	if lastOperation == nil {
		return false
	}

	return lastOperation.State == gardencorev1beta1.LastOperationStateFailed
}

func lastOperationStateChanged(oldLastOp, newLastOp *gardencorev1beta1.LastOperation) bool {
	if newLastOp == nil {
		return false
	}

	newLastOperationStateSucceededOrErroneous := newLastOp.State == gardencorev1beta1.LastOperationStateSucceeded || newLastOp.State == gardencorev1beta1.LastOperationStateError || newLastOp.State == gardencorev1beta1.LastOperationStateFailed

	if newLastOperationStateSucceededOrErroneous {
		if oldLastOp != nil {
			return !reflect.DeepEqual(oldLastOp, newLastOp)
		}
		return true
	}

	return false
}

// GetExtensionLastOperation returns the LastOperation of the passed extension object.
func GetExtensionLastOperation(obj client.Object) *gardencorev1beta1.LastOperation {
	acc, err := extensions.Accessor(obj)
	if err != nil {
		return nil
	}

	return acc.GetExtensionStatus().GetLastOperation()
}

// HasType filters the incoming OperatingSystemConfigs for ones that have the same type
// as the given type.
func HasType(typeName string) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		acc, err := extensions.Accessor(obj)
		if err != nil {
			return false
		}

		return acc.GetExtensionSpec().GetExtensionType() == typeName
	})
}

// AddTypeAndClassPredicates returns a new slice which contains a HasClass, a type predicate and the given `predicates`.
// If more than one extensionTypes is given they are combined with an OR.
func AddTypeAndClassPredicates(predicates []predicate.Predicate, extensionClass extensionsv1alpha1.ExtensionClass, extensionTypes ...string) []predicate.Predicate {
	resultPredicates := make([]predicate.Predicate, 0, len(predicates)+2)
	resultPredicates = append(resultPredicates, HasClass(extensionClass))
	resultPredicates = append(resultPredicates, HasOneOfTypesPredicate(extensionTypes...))
	resultPredicates = append(resultPredicates, predicates...)
	return resultPredicates
}

// HasOneOfTypesPredicate returns a new slice which contains a type predicate.
// If more than one extensionTypes is given they are combined with an OR.
func HasOneOfTypesPredicate(extensionTypes ...string) predicate.Predicate {
	if len(extensionTypes) == 1 {
		return HasType(extensionTypes[0])
	}

	orPreds := make([]predicate.Predicate, 0, len(extensionTypes))
	for _, extensionType := range extensionTypes {
		orPreds = append(orPreds, HasType(extensionType))
	}

	return predicate.Or(orPreds...)
}

// HasClass filters the incoming objects for the given extension classes.
// For backwards compatibility, if the class is unset in the extension object, it is assumed that the extension belongs to a shoot cluster.
// An empty 'extensionClass' is likewise treated to be of class 'shoot'.
func HasClass(extensionClasses ...extensionsv1alpha1.ExtensionClass) predicate.Predicate {
	if len(extensionClasses) == 0 {
		extensionClasses = append(extensionClasses, extensionsv1alpha1.ExtensionClassShoot)
	} else if len(extensionClasses) == 1 && extensionClasses[0] == "" {
		extensionClasses[0] = extensionsv1alpha1.ExtensionClassShoot
	}

	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if obj == nil {
			return false
		}

		accessor, err := extensions.Accessor(obj)
		if err != nil {
			return false
		}

		return set.New(extensionClasses...).Has(extensionsv1alpha1helper.GetExtensionClassOrDefault(accessor.GetExtensionSpec().GetExtensionClass()))
	})
}

// HasPurpose filters the incoming ControlPlanes for the given spec.purpose.
//
// Deprecated: Purpose field is being deprecated and will be removed in gardener v1.123.0.
// The value "exposure" is no longer used since the enablement of SNI, and the value "normal" is redundant.
// TODO(theoddora): Remove this function in v1.123.0 when the Purpose field is removed.
func HasPurpose(purpose extensionsv1alpha1.Purpose) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		controlPlane, ok := obj.(*extensionsv1alpha1.ControlPlane)
		if !ok {
			return false
		}

		// needed because ControlPlane of type "normal" has the spec.purpose field not set
		if controlPlane.Spec.Purpose == nil && purpose == extensionsv1alpha1.Normal {
			return true
		}

		if controlPlane.Spec.Purpose == nil {
			return false
		}

		return *controlPlane.Spec.Purpose == purpose
	})
}
