// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
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

// Not inverts the passed predicate.
func Not(p predicate.Predicate) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return !p.Create(event)
		},
		UpdateFunc: func(event event.UpdateEvent) bool {
			return !p.Update(event)
		},
		GenericFunc: func(event event.GenericEvent) bool {
			return !p.Generic(event)
		},
		DeleteFunc: func(event event.DeleteEvent) bool {
			return !p.Delete(event)
		},
	}
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
		CreateFunc:  func(e event.CreateEvent) bool { return has(Create) },
		UpdateFunc:  func(e event.UpdateEvent) bool { return has(Update) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return has(Delete) },
		GenericFunc: func(e event.GenericEvent) bool { return has(Generic) },
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
					gardencorev1beta1helper.GetCondition(oldConditions, condition),
					gardencorev1beta1helper.GetCondition(newConditions, condition),
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
