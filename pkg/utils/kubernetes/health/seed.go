// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var (
	trueSeedConditionTypes = []gardencorev1beta1.ConditionType{
		gardencorev1beta1.GardenletReady,
		gardencorev1beta1.SeedSystemComponentsHealthy,
	}
)

// CheckSeed checks if the Seed is up-to-date and if its extensions have been successfully bootstrapped.
func CheckSeed(seed *gardencorev1beta1.Seed, identity *gardencorev1beta1.Gardener) error {
	if err := CheckSeedIsUpToDate(seed, identity); err != nil {
		return err
	}

	return checkSeedConditions(seed)
}

// CheckSeedIsUpToDate checks if the Seed's observed Gardener version and generation are up-to-date compared to the provided identity.
func CheckSeedIsUpToDate(seed *gardencorev1beta1.Seed, identity *gardencorev1beta1.Gardener) error {
	if !equality.Semantic.DeepEqual(seed.Status.Gardener, identity) {
		return fmt.Errorf("observing Gardener version not up to date (%v/%v)", seed.Status.Gardener, identity)
	}

	if seed.Status.ObservedGeneration < seed.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", seed.Status.ObservedGeneration, seed.Generation)
	}

	return nil
}

// CheckRequiredSeedConditions checks if the required conditions of the Seed are set to True and returns the conditions that are not in the expected state.
// It returns an error if any of the required conditions is missing.
func CheckRequiredSeedConditions(seed *gardencorev1beta1.Seed) ([]gardencorev1beta1.Condition, error) {
	var failedConditionTypes []gardencorev1beta1.Condition

	for _, trueConditionType := range trueSeedConditionTypes {
		conditionType := string(trueConditionType)
		condition := v1beta1helper.GetCondition(seed.Status.Conditions, trueConditionType)
		if condition == nil {
			return nil, requiredConditionMissing(conditionType)
		}
		if err := checkConditionState(string(condition.Type), string(gardencorev1beta1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			failedConditionTypes = append(failedConditionTypes, *condition)
		}
	}

	return failedConditionTypes, nil
}

// CheckSeedForMigration checks if the Seed is up-to-date (comparing only the versions) and if its extensions have been successfully bootstrapped.
func CheckSeedForMigration(seed *gardencorev1beta1.Seed, identity *gardencorev1beta1.Gardener) error {
	if seed.Status.Gardener == nil || identity == nil {
		return fmt.Errorf("missing Gardener version information on source or destination seed")
	}
	if seed.Status.Gardener.Version != identity.Version {
		return fmt.Errorf("observing Gardener version not up to date (%s/%s)", seed.Status.Gardener.Version, identity.Version)
	}
	if seed.Status.ObservedGeneration < seed.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", seed.Status.ObservedGeneration, seed.Generation)
	}

	return checkSeedConditions(seed)
}

// checkSeedConditions checks if the .status.observedGeneration field is not outdated and if the Seed's extensions have been
// successfully bootstrapped.
func checkSeedConditions(seed *gardencorev1beta1.Seed) error {
	falseConditions, err := CheckRequiredSeedConditions(seed)
	if err != nil {
		return err
	}
	for _, condition := range falseConditions {
		if err := checkConditionState(string(condition.Type), string(gardencorev1beta1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
}
