// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

// CheckManagedResourceProgressing checks if the condition ResourcesProgressing of a ManagedResource is False.
func CheckManagedResourceProgressing(mr *resourcesv1alpha1.ManagedResource) error {
	status := mr.Status
	conditionProgressing := v1beta1helper.GetCondition(status.Conditions, resourcesv1alpha1.ResourcesProgressing)

	if conditionProgressing == nil {
		return fmt.Errorf("condition %s for managed resource %s/%s has not been reported yet", resourcesv1alpha1.ResourcesProgressing, mr.GetNamespace(), mr.GetName())
	} else if conditionProgressing.Status != gardencorev1beta1.ConditionFalse {
		return fmt.Errorf("condition %s of managed resource %s/%s is %s: %s", resourcesv1alpha1.ResourcesProgressing, mr.GetNamespace(), mr.GetName(), conditionProgressing.Status, conditionProgressing.Message)
	}

	return nil
}
