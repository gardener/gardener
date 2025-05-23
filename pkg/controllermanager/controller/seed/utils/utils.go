// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
)

// GetThresholdForCondition returns the threshold duration from the configuration for the provided condition type.
func GetThresholdForCondition(conditions []controllermanagerconfigv1alpha1.ConditionThreshold, conditionType gardencorev1beta1.ConditionType) time.Duration {
	for _, threshold := range conditions {
		if threshold.Type == string(conditionType) {
			return threshold.Duration.Duration
		}
	}
	return 0
}

// SetToProgressingOrUnknown sets the provided condition to Progressing or to Unknown based on whether the provided
// conditionThreshold has passed compared to the condition's last transition time.
func SetToProgressingOrUnknown(
	clock clock.Clock,
	conditionThreshold time.Duration,
	condition gardencorev1beta1.Condition,
	reason, message string,
	codes ...gardencorev1beta1.ErrorCode,
) gardencorev1beta1.Condition {
	return setToProgressingIfWithinThreshold(clock, conditionThreshold, condition, gardencorev1beta1.ConditionUnknown, reason, message, codes...)
}

// SetToProgressingOrFalse sets the provided condition to Progressing or to False based on whether the provided
// conditionThreshold has passed compared to the condition's last transition time.
func SetToProgressingOrFalse(
	clock clock.Clock,
	conditionThreshold time.Duration,
	condition gardencorev1beta1.Condition,
	reason, message string,
	codes ...gardencorev1beta1.ErrorCode,
) gardencorev1beta1.Condition {
	return setToProgressingIfWithinThreshold(clock, conditionThreshold, condition, gardencorev1beta1.ConditionFalse, reason, message, codes...)
}

func setToProgressingIfWithinThreshold(
	clock clock.Clock,
	conditionThreshold time.Duration,
	condition gardencorev1beta1.Condition,
	eventualConditionStatus gardencorev1beta1.ConditionStatus,
	reason, message string,
	codes ...gardencorev1beta1.ErrorCode,
) gardencorev1beta1.Condition {
	switch condition.Status {
	case gardencorev1beta1.ConditionTrue:
		if conditionThreshold == 0 {
			return v1beta1helper.UpdatedConditionWithClock(clock, condition, eventualConditionStatus, reason, message, codes...)
		}
		return v1beta1helper.UpdatedConditionWithClock(clock, condition, gardencorev1beta1.ConditionProgressing, reason, message, codes...)

	case gardencorev1beta1.ConditionProgressing:
		if conditionThreshold == 0 {
			return v1beta1helper.UpdatedConditionWithClock(clock, condition, eventualConditionStatus, reason, message, codes...)
		}

		if delta := clock.Now().UTC().Sub(condition.LastTransitionTime.UTC()); delta <= conditionThreshold {
			return v1beta1helper.UpdatedConditionWithClock(clock, condition, gardencorev1beta1.ConditionProgressing, reason, message, codes...)
		}
		return v1beta1helper.UpdatedConditionWithClock(clock, condition, eventualConditionStatus, reason, message, codes...)
	}

	return v1beta1helper.UpdatedConditionWithClock(clock, condition, eventualConditionStatus, reason, message, codes...)
}

// PatchSeedCondition patches the seed conditions in case they need to be updated.
func PatchSeedCondition(ctx context.Context, log logr.Logger, c client.StatusWriter, seed *gardencorev1beta1.Seed, condition gardencorev1beta1.Condition) error {
	patch := client.StrategicMergeFrom(seed.DeepCopy())

	conditions := v1beta1helper.MergeConditions(seed.Status.Conditions, condition)
	if !v1beta1helper.ConditionsNeedUpdate(seed.Status.Conditions, conditions) {
		return nil
	}

	seed.Status.Conditions = conditions
	if err := c.Patch(ctx, seed, patch); err != nil {
		return err
	}

	log.Info("Successfully patched condition", "conditionType", condition.Type, "conditionStatus", condition.Status)
	return nil
}
