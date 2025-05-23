// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

func requiredConditionMissing(conditionType string) error {
	return fmt.Errorf("condition %q is missing", conditionType)
}

func checkConditionState(conditionType, expected, actual, reason, message string) error {
	if expected != actual {
		return fmt.Errorf("condition %q has invalid status %s (expected %s) due to %s: %s",
			conditionType, actual, expected, reason, message)
	}
	return nil
}

// ObjectHasAnnotationWithValue returns a health check function that checks if a given Object has an annotation with
// a specified value.
func ObjectHasAnnotationWithValue(key, value string) Func {
	return func(o client.Object) error {
		actual, ok := o.GetAnnotations()[key]
		if !ok {
			return fmt.Errorf("object does not have %q annotation", key)
		}
		if actual != value {
			return fmt.Errorf("object's %q annotation is not %q but %q", key, value, actual)
		}
		return nil
	}
}

// ConditionerFunc to update a condition with type and message
type conditionerFunc func(conditionType string, message string) gardencorev1beta1.Condition
