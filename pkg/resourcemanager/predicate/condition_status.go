// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

// ConditionChangeFn is a type for comparing two conditions.
type ConditionChangeFn func(con1, con2 *gardencorev1beta1.Condition) bool

// DefaultConditionChange compares the given conditions and returns `true` if the `Status` has changed.
var DefaultConditionChange ConditionChangeFn = func(con1, con2 *gardencorev1beta1.Condition) bool {
	if con1 == nil {
		// trigger if condition was added
		return con2 != nil
	}

	if con2 == nil {
		return true // condition was removed
	}

	return con1.Status != con2.Status
}

// ConditionChangedToUnhealthy compares the given conditions and returns `true` if the `Status` has changed to an unhealthy state.
var ConditionChangedToUnhealthy ConditionChangeFn = func(con1, con2 *gardencorev1beta1.Condition) bool {
	return (con1 == nil || con1.Status == gardencorev1beta1.ConditionTrue) &&
		(con2 != nil && con2.Status == gardencorev1beta1.ConditionFalse)
}

// ConditionStatusChanged is a predicate that detects changes to the status of a Condition with a given type.
func ConditionStatusChanged(conditionType gardencorev1beta1.ConditionType, changeFn ConditionChangeFn) predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld == nil {
				return false
			}
			if e.ObjectNew == nil {
				return false
			}

			old, ok := e.ObjectOld.(*resourcesv1alpha1.ManagedResource)
			if !ok {
				return false
			}
			new, ok := e.ObjectNew.(*resourcesv1alpha1.ManagedResource)
			if !ok {
				return false
			}

			oldCondition := v1beta1helper.GetCondition(old.Status.Conditions, conditionType)
			newCondition := v1beta1helper.GetCondition(new.Status.Conditions, conditionType)

			return changeFn(oldCondition, newCondition)
		},
	}
}
