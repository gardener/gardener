// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	"k8s.io/utils/clock"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

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
		if delta := clock.Now().UTC().Sub(condition.LastTransitionTime.Time.UTC()); delta <= threshold {
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
