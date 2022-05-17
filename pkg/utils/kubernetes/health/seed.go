// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package health

import (
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"

	"k8s.io/apimachinery/pkg/api/equality"
)

var (
	trueSeedConditionTypes = []gardencorev1beta1.ConditionType{
		gardencorev1beta1.SeedGardenletReady,
		gardencorev1beta1.SeedBootstrapped,
		gardencorev1beta1.SeedSystemComponentsHealthy,
	}
)

// CheckSeed checks if the Seed is up-to-date and if its extensions have been successfully bootstrapped.
func CheckSeed(seed *gardencorev1beta1.Seed, identity *gardencorev1beta1.Gardener) error {
	if !equality.Semantic.DeepEqual(seed.Status.Gardener, identity) {
		return fmt.Errorf("observing Gardener version not up to date (%v/%v)", seed.Status.Gardener, identity)
	}

	return checkSeed(seed)
}

// CheckSeedForMigration checks if the Seed is up-to-date (comparing only the versions) and if its extensions have been successfully bootstrapped.
func CheckSeedForMigration(seed *gardencorev1beta1.Seed, identity *gardencorev1beta1.Gardener) error {
	if seed.Status.Gardener.Version != identity.Version {
		return fmt.Errorf("observing Gardener version not up to date (%s/%s)", seed.Status.Gardener.Version, identity.Version)
	}

	return checkSeed(seed)
}

// checkSeed checks if the seed.Status.ObservedGeneration ObservedGeneration is not outdated and if its extensions have been successfully bootstrapped.
func checkSeed(seed *gardencorev1beta1.Seed) error {
	if seed.Status.ObservedGeneration < seed.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", seed.Status.ObservedGeneration, seed.Generation)
	}

	for _, trueConditionType := range trueSeedConditionTypes {
		conditionType := string(trueConditionType)
		condition := v1beta1helper.GetCondition(seed.Status.Conditions, trueConditionType)
		if condition == nil {
			return requiredConditionMissing(conditionType)
		}
		if err := checkConditionState(string(condition.Type), string(gardencorev1beta1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
}
