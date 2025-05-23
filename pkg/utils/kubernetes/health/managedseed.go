// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
)

var (
	managedSeedConditionTypes = []gardencorev1beta1.ConditionType{
		seedmanagementv1alpha1.ManagedSeedShootReconciled,
		seedmanagementv1alpha1.SeedRegistered,
	}
)

// CheckManagedSeed checks if the given ManagedSeed is up-to-date and if its Seed has been registered.
func CheckManagedSeed(managedSeed *seedmanagementv1alpha1.ManagedSeed) error {
	if managedSeed.Status.ObservedGeneration < managedSeed.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", managedSeed.Status.ObservedGeneration, managedSeed.Generation)
	}

	for _, conditionType := range managedSeedConditionTypes {
		condition := v1beta1helper.GetCondition(managedSeed.Status.Conditions, conditionType)
		if condition == nil {
			return requiredConditionMissing(string(conditionType))
		}
		if err := checkConditionState(string(condition.Type), string(gardencorev1beta1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
}
