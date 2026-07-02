// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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

// Clock is an alias for the clock.Clock interface, which is used to get the current time. Exposed for tests.
var Clock clock.Clock = clock.RealClock{}

// IsSkippedUntil returns true when obj carries the AnnotationCareSkipHealthChecksUntil annotation
// with a valid RFC3339 timestamp that lies in the future. An absent, invalid, or past value returns false.
func IsSkippedUntil(obj metav1.Object) bool {
	val, ok := obj.GetAnnotations()[v1beta1constants.AnnotationCareSkipHealthChecksUntil]
	if !ok {
		return false
	}
	t, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return false
	}
	return Clock.Now().Before(t)
}
