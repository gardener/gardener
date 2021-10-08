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
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

// CheckManagedResource checks if all conditions of a ManagedResource ('ResourcesApplied' and 'ResourcesHealthy')
// are True and .status.observedGeneration matches the current .metadata.generation
func CheckManagedResource(mr *resourcesv1alpha1.ManagedResource) error {
	if err := CheckManagedResourceApplied(mr); err != nil {
		return err
	}

	return CheckManagedResourceHealthy(mr)
}

// CheckManagedResourceApplied checks if the condition 'ResourcesApplied' of a ManagedResource
// is True and the .status.observedGeneration matches the current .metadata.generation
func CheckManagedResourceApplied(mr *resourcesv1alpha1.ManagedResource) error {
	status := mr.Status
	if status.ObservedGeneration != mr.GetGeneration() {
		return fmt.Errorf("observed generation of managed resource %s/%s outdated (%d/%d)", mr.GetNamespace(), mr.GetName(), status.ObservedGeneration, mr.GetGeneration())
	}

	conditionApplied := v1beta1helper.GetCondition(status.Conditions, resourcesv1alpha1.ResourcesApplied)

	if conditionApplied == nil {
		return fmt.Errorf("condition %s for managed resource %s/%s has not been reported yet", resourcesv1alpha1.ResourcesApplied, mr.GetNamespace(), mr.GetName())
	}
	if conditionApplied.Status != gardencorev1beta1.ConditionTrue {
		return fmt.Errorf("condition %s of managed resource %s/%s is %s: %s", resourcesv1alpha1.ResourcesApplied, mr.GetNamespace(), mr.GetName(), conditionApplied.Status, conditionApplied.Message)
	}

	return nil
}

// CheckManagedResourceHealthy checks if the condition 'ResourcesHealthy' of a ManagedResource is True
func CheckManagedResourceHealthy(mr *resourcesv1alpha1.ManagedResource) error {
	status := mr.Status
	conditionHealthy := v1beta1helper.GetCondition(status.Conditions, resourcesv1alpha1.ResourcesHealthy)

	if conditionHealthy == nil {
		return fmt.Errorf("condition %s for managed resource %s/%s has not been reported yet", resourcesv1alpha1.ResourcesHealthy, mr.GetNamespace(), mr.GetName())
	} else if conditionHealthy.Status != gardencorev1beta1.ConditionTrue {
		return fmt.Errorf("condition %s of managed resource %s/%s is %s: %s", resourcesv1alpha1.ResourcesHealthy, mr.GetNamespace(), mr.GetName(), conditionHealthy.Status, conditionHealthy.Message)
	}

	return nil
}
