// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// Now determines the current metav1.Time.
var Now = metav1.Now

// InitCondition initializes a new Condition with an Unknown status.
func InitCondition(conditionType gardencorev1alpha1.ConditionType) gardencorev1alpha1.Condition {
	return gardencorev1alpha1.Condition{
		Type:               conditionType,
		Status:             gardencorev1alpha1.ConditionUnknown,
		Reason:             "ConditionInitialized",
		Message:            "The condition has been initialized but its semantic check has not been performed yet.",
		LastTransitionTime: Now(),
		LastUpdateTime:     Now(),
	}
}

// GetCondition returns the condition with the given <conditionType> out of the list of <conditions>.
// In case the required type could not be found, it returns nil.
func GetCondition(conditions []gardencorev1alpha1.Condition, conditionType gardencorev1alpha1.ConditionType) *gardencorev1alpha1.Condition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			c := condition
			return &c
		}
	}
	return nil
}

// GetOrInitCondition tries to retrieve the condition with the given condition type from the given conditions.
// If the condition could not be found, it returns an initialized condition of the given type.
func GetOrInitCondition(conditions []gardencorev1alpha1.Condition, conditionType gardencorev1alpha1.ConditionType) gardencorev1alpha1.Condition {
	if condition := GetCondition(conditions, conditionType); condition != nil {
		return *condition
	}
	return InitCondition(conditionType)
}

// UpdatedCondition updates the properties of one specific condition.
func UpdatedCondition(condition gardencorev1alpha1.Condition, status gardencorev1alpha1.ConditionStatus, reason, message string, codes ...gardencorev1alpha1.ErrorCode) gardencorev1alpha1.Condition {
	builder, err := NewConditionBuilder(condition.Type)
	utilruntime.Must(err)
	newCondition, _ := builder.
		WithOldCondition(condition).
		WithStatus(status).
		WithReason(reason).
		WithMessage(message).
		WithCodes(codes...).
		Build()

	return newCondition
}

// MergeConditions merges the given <oldConditions> with the <newConditions>. Existing conditions are superseded by
// the <newConditions> (depending on the condition type).
func MergeConditions(oldConditions []gardencorev1alpha1.Condition, newConditions ...gardencorev1alpha1.Condition) []gardencorev1alpha1.Condition {
	var (
		out         = make([]gardencorev1alpha1.Condition, 0, len(oldConditions)+len(newConditions))
		typeToIndex = make(map[gardencorev1alpha1.ConditionType]int, len(oldConditions))
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
