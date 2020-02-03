// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package helper

import (
	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Now determines the current metav1.Time.
var Now = metav1.Now

// InitCondition initializes a new Condition with an Unknown status.
func InitCondition(conditionType resourcesv1alpha1.ConditionType) resourcesv1alpha1.ManagedResourceCondition {
	return resourcesv1alpha1.ManagedResourceCondition{
		Type:               conditionType,
		Status:             resourcesv1alpha1.ConditionUnknown,
		Reason:             "ConditionInitialized",
		Message:            "The condition has been initialized but its semantic check has not been performed yet.",
		LastTransitionTime: Now(),
	}
}

// GetCondition returns the condition with the given <conditionType> out of the list of <conditions>.
// In case the required type could not be found, it returns nil.
func GetCondition(conditions []resourcesv1alpha1.ManagedResourceCondition, conditionType resourcesv1alpha1.ConditionType) *resourcesv1alpha1.ManagedResourceCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}

	return nil
}

// GetOrInitCondition tries to retrieve the condition with the given condition type from the given conditions.
// If the condition could not be found, it returns an initialized condition of the given type.
func GetOrInitCondition(conditions []resourcesv1alpha1.ManagedResourceCondition, conditionType resourcesv1alpha1.ConditionType) resourcesv1alpha1.ManagedResourceCondition {
	if condition := GetCondition(conditions, conditionType); condition != nil {
		return *condition
	}

	return InitCondition(conditionType)
}

// UpdatedCondition updates the properties of one specific condition.
func UpdatedCondition(condition resourcesv1alpha1.ManagedResourceCondition, status resourcesv1alpha1.ConditionStatus, reason, message string) resourcesv1alpha1.ManagedResourceCondition {
	newCondition := resourcesv1alpha1.ManagedResourceCondition{
		Type:               condition.Type,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: condition.LastTransitionTime,
		LastUpdateTime:     Now(),
	}

	if condition.Status != status {
		newCondition.LastTransitionTime = Now()
	}

	return newCondition
}

// MergeConditions merges the given <oldConditions> with the <newConditions>. Existing conditions are superseded by
// the <newConditions> (depending on the condition type).
func MergeConditions(oldConditions []resourcesv1alpha1.ManagedResourceCondition, newConditions ...resourcesv1alpha1.ManagedResourceCondition) []resourcesv1alpha1.ManagedResourceCondition {
	var (
		out         = make([]resourcesv1alpha1.ManagedResourceCondition, 0, len(oldConditions))
		typeToIndex = make(map[resourcesv1alpha1.ConditionType]int, len(oldConditions))
	)

	for i, condition := range oldConditions {
		out = append(out, condition)
		typeToIndex[condition.Type] = i
	}

	for _, condition := range newConditions {
		if index, ok := typeToIndex[condition.Type]; ok {
			out[index] = condition
			continue
		}
		out = append(out, condition)
	}

	return out
}
