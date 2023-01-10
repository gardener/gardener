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
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
)

var (
	managedSeedConditionTypes = []gardencorev1beta1.ConditionType{
		seedmanagementv1alpha1.ManagedSeedShootReconciled,
		seedmanagementv1alpha1.ManagedSeedSeedRegistered,
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
