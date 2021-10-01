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

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"

	corev1 "k8s.io/api/core/v1"
)

var (
	trueManagedResourceConditionTypes = []resourcesv1alpha1.ConditionType{
		resourcesv1alpha1.ResourcesApplied,
		resourcesv1alpha1.ResourcesHealthy,
	}
)

// CheckManagedResource checks whether the given ManagedResource is healthy.
// A ManagedResource is considered healthy if its controller observed its current revision,
// and if the required conditions are healthy.
func CheckManagedResource(managedResource *resourcesv1alpha1.ManagedResource) error {
	if managedResource.Status.ObservedGeneration < managedResource.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", managedResource.Status.ObservedGeneration, managedResource.Generation)
	}

	for _, trueConditionType := range trueManagedResourceConditionTypes {
		conditionType := string(trueConditionType)
		condition := getManagedResourceCondition(managedResource.Status.Conditions, trueConditionType)
		if condition == nil {
			return requiredConditionMissing(conditionType)
		}
		if err := checkConditionState(string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
}

func getManagedResourceCondition(conditions []resourcesv1alpha1.ManagedResourceCondition, conditionType resourcesv1alpha1.ConditionType) *resourcesv1alpha1.ManagedResourceCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}
