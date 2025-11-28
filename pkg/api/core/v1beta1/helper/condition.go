// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// InitConditionWithClock initializes a new Condition with an Unknown status. It allows passing a custom clock for testing.
func InitConditionWithClock(clock clock.Clock, conditionType gardencorev1beta1.ConditionType) gardencorev1beta1.Condition {
	now := metav1.Time{Time: clock.Now()}
	return gardencorev1beta1.Condition{
		Type:               conditionType,
		Status:             gardencorev1beta1.ConditionUnknown,
		Reason:             "ConditionInitialized",
		Message:            "The condition has been initialized but its semantic check has not been performed yet.",
		LastTransitionTime: now,
		LastUpdateTime:     now,
	}
}

// GetCondition returns the condition with the given <conditionType> out of the list of <conditions>.
// In case the required type could not be found, it returns nil.
func GetCondition(conditions []gardencorev1beta1.Condition, conditionType gardencorev1beta1.ConditionType) *gardencorev1beta1.Condition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			c := condition
			return &c
		}
	}
	return nil
}

// GetOrInitConditionWithClock tries to retrieve the condition with the given condition type from the given conditions.
// If the condition could not be found, it returns an initialized condition of the given type. It allows passing a custom clock for testing.
func GetOrInitConditionWithClock(clock clock.Clock, conditions []gardencorev1beta1.Condition, conditionType gardencorev1beta1.ConditionType) gardencorev1beta1.Condition {
	if condition := GetCondition(conditions, conditionType); condition != nil {
		return *condition
	}
	return InitConditionWithClock(clock, conditionType)
}

// FailedCondition returns a progressing or false condition depending on the progressing threshold.
func FailedCondition(
	clock clock.Clock,
	lastOperation *gardencorev1beta1.LastOperation,
	conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration,
	condition gardencorev1beta1.Condition,
	reason string,
	message string,
	codes ...gardencorev1beta1.ErrorCode,
) gardencorev1beta1.Condition {
	switch condition.Status {
	case gardencorev1beta1.ConditionTrue:
		if _, ok := conditionThresholds[condition.Type]; !ok {
			return UpdatedConditionWithClock(clock, condition, gardencorev1beta1.ConditionFalse, reason, message, codes...)
		}
		return UpdatedConditionWithClock(clock, condition, gardencorev1beta1.ConditionProgressing, reason, message, codes...)

	case gardencorev1beta1.ConditionProgressing:
		threshold, ok := conditionThresholds[condition.Type]
		if !ok {
			return UpdatedConditionWithClock(clock, condition, gardencorev1beta1.ConditionFalse, reason, message, codes...)
		}
		if lastOperation != nil && lastOperation.State == gardencorev1beta1.LastOperationStateSucceeded && clock.Now().UTC().Sub(lastOperation.LastUpdateTime.UTC()) <= threshold {
			return UpdatedConditionWithClock(clock, condition, gardencorev1beta1.ConditionProgressing, reason, message, codes...)
		}
		if delta := clock.Now().UTC().Sub(condition.LastTransitionTime.UTC()); delta <= threshold {
			return UpdatedConditionWithClock(clock, condition, gardencorev1beta1.ConditionProgressing, reason, message, codes...)
		}
		return UpdatedConditionWithClock(clock, condition, gardencorev1beta1.ConditionFalse, reason, message, codes...)

	case gardencorev1beta1.ConditionFalse:
		threshold, ok := conditionThresholds[condition.Type]
		if ok &&
			((lastOperation != nil && lastOperation.State == gardencorev1beta1.LastOperationStateSucceeded && clock.Now().UTC().Sub(lastOperation.LastUpdateTime.UTC()) <= threshold) ||
				(reason != condition.Reason)) {
			return UpdatedConditionWithClock(clock, condition, gardencorev1beta1.ConditionProgressing, reason, message, codes...)
		}
	}

	return UpdatedConditionWithClock(clock, condition, gardencorev1beta1.ConditionFalse, reason, message, codes...)
}

// UpdatedConditionWithClock updates the properties of one specific condition. It allows passing a custom clock for testing.
func UpdatedConditionWithClock(clock clock.Clock, condition gardencorev1beta1.Condition, status gardencorev1beta1.ConditionStatus, reason, message string, codes ...gardencorev1beta1.ErrorCode) gardencorev1beta1.Condition {
	builder, err := NewConditionBuilder(condition.Type)
	utilruntime.Must(err)

	newCondition, _ := builder.
		WithOldCondition(condition).
		WithClock(clock).
		WithStatus(status).
		WithReason(reason).
		WithMessage(message).
		WithCodes(codes...).
		Build()

	return newCondition
}

// UpdatedConditionUnknownErrorWithClock updates the condition to 'Unknown' status and the message of the given error. It allows passing a custom clock for testing.
func UpdatedConditionUnknownErrorWithClock(clock clock.Clock, condition gardencorev1beta1.Condition, err error, codes ...gardencorev1beta1.ErrorCode) gardencorev1beta1.Condition {
	message := "unknown"
	if err != nil {
		message = err.Error()
	}
	return UpdatedConditionUnknownErrorMessageWithClock(clock, condition, message, codes...)
}

// UpdatedConditionUnknownErrorMessageWithClock updates the condition with 'Unknown' status and the given message. It allows passing a custom clock for testing.
func UpdatedConditionUnknownErrorMessageWithClock(clock clock.Clock, condition gardencorev1beta1.Condition, message string, codes ...gardencorev1beta1.ErrorCode) gardencorev1beta1.Condition {
	return UpdatedConditionWithClock(clock, condition, gardencorev1beta1.ConditionUnknown, gardencorev1beta1.ConditionCheckError, message, codes...)
}

// BuildConditions builds and returns the conditions using the given conditions as a base,
// by first removing all conditions with the given types and then merging the given new conditions (which must be of the same types).
func BuildConditions(baseConditions, newConditions []gardencorev1beta1.Condition, removeConditionTypes []gardencorev1beta1.ConditionType) []gardencorev1beta1.Condition {
	result := RemoveConditions(baseConditions, removeConditionTypes...)
	result = MergeConditions(result, newConditions...)
	return result
}

// MergeConditions merges the given <oldConditions> with the <newConditions>. Existing conditions are superseded by
// the <newConditions> (depending on the condition type).
func MergeConditions(oldConditions []gardencorev1beta1.Condition, newConditions ...gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
	var (
		out         = make([]gardencorev1beta1.Condition, 0, len(oldConditions))
		typeToIndex = make(map[gardencorev1beta1.ConditionType]int, len(oldConditions))
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

// RemoveConditions removes the conditions with the given types from the given conditions slice.
func RemoveConditions(conditions []gardencorev1beta1.Condition, conditionTypes ...gardencorev1beta1.ConditionType) []gardencorev1beta1.Condition {
	unwantedConditionTypes := sets.New(conditionTypes...)

	var newConditions []gardencorev1beta1.Condition
	for _, condition := range conditions {
		if !unwantedConditionTypes.Has(condition.Type) {
			newConditions = append(newConditions, condition)
		}
	}

	return newConditions
}

// RetainConditions retains all given conditionsTypes from the given conditions slice.
func RetainConditions(conditions []gardencorev1beta1.Condition, conditionTypes ...gardencorev1beta1.ConditionType) []gardencorev1beta1.Condition {
	wantedConditionsTypes := sets.New(conditionTypes...)

	var newConditions []gardencorev1beta1.Condition
	for _, condition := range conditions {
		if wantedConditionsTypes.Has(condition.Type) {
			newConditions = append(newConditions, condition)
		}
	}

	return newConditions
}

// ConditionsNeedUpdate returns true if the <existingConditions> must be updated based on <newConditions>.
func ConditionsNeedUpdate(existingConditions, newConditions []gardencorev1beta1.Condition) bool {
	return existingConditions == nil || !apiequality.Semantic.DeepEqual(newConditions, existingConditions)
}

// NewConditionOrError returns the given new condition or returns an unknown error condition if an error occurred or `newCondition` is nil.
func NewConditionOrError(clock clock.Clock, oldCondition gardencorev1beta1.Condition, newCondition *gardencorev1beta1.Condition, err error) gardencorev1beta1.Condition {
	if err != nil || newCondition == nil {
		return UpdatedConditionUnknownErrorWithClock(clock, oldCondition, err)
	}
	return *newCondition
}
