// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"fmt"
	"strings"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
)

var (
	trueMachineDeploymentConditionTypes = []machinev1alpha1.MachineDeploymentConditionType{
		machinev1alpha1.MachineDeploymentAvailable,
	}

	trueOptionalMachineDeploymentConditionTypes = []machinev1alpha1.MachineDeploymentConditionType{
		machinev1alpha1.MachineDeploymentProgressing,
	}

	falseMachineDeploymentConditionTypes = []machinev1alpha1.MachineDeploymentConditionType{
		machinev1alpha1.MachineDeploymentReplicaFailure,
		machinev1alpha1.MachineDeploymentFrozen,
	}
)

// CheckMachineDeployment checks whether the given MachineDeployment is healthy.
// A MachineDeployment is considered healthy if its controller observed its current revision and if its desired number
// of replicas is equal to its updated replicas.
func CheckMachineDeployment(deployment *machinev1alpha1.MachineDeployment) error {
	if deployment.Status.ObservedGeneration < deployment.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", deployment.Status.ObservedGeneration, deployment.Generation)
	}

	for _, trueConditionType := range trueMachineDeploymentConditionTypes {
		condition := getMachineDeploymentCondition(deployment.Status.Conditions, trueConditionType)
		if condition == nil {
			return requiredConditionMissing(string(trueConditionType))
		}

		if err := checkConditionStatus(condition, machinev1alpha1.ConditionTrue); err != nil {
			return err
		}
	}

	for _, trueOptionalConditionType := range trueOptionalMachineDeploymentConditionTypes {
		condition := getMachineDeploymentCondition(deployment.Status.Conditions, trueOptionalConditionType)
		if condition == nil {
			continue
		}

		isStrategyManualInPlace := deployment.Spec.Strategy.Type == machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType && deployment.Spec.Strategy.InPlaceUpdate != nil && deployment.Spec.Strategy.InPlaceUpdate.OrchestrationType == machinev1alpha1.OrchestrationTypeManual
		if isStrategyManualInPlace && trueOptionalConditionType == machinev1alpha1.MachineDeploymentProgressing && condition.Status == machinev1alpha1.ConditionTrue && deployment.Status.UpdatedReplicas != deployment.Status.Replicas {
			// In case of manual in-place update, the MachineDeployment is considered healthy if the updated replicas are not equal to the replicas.
			// This is because the updated replicas are not increased until the user selects remaining machines from the old machine set for update.
			// In this case, the MachineDeployment is considered healthy if the progressing condition is true.
			continue
		}

		if err := checkConditionStatus(condition, machinev1alpha1.ConditionTrue); err != nil {
			return err
		}
	}

	for _, falseConditionType := range falseMachineDeploymentConditionTypes {
		condition := getMachineDeploymentCondition(deployment.Status.Conditions, falseConditionType)
		if condition == nil {
			continue
		}

		if err := checkConditionStatus(condition, machinev1alpha1.ConditionFalse); err != nil {
			return err
		}
	}

	return nil
}

func checkConditionStatus(condition *machinev1alpha1.MachineDeploymentCondition, expectedStatus machinev1alpha1.ConditionStatus) error {
	if condition.Status != expectedStatus {
		return fmt.Errorf("%s (%s)", strings.Trim(condition.Message, "."), condition.Reason)
	}
	return nil
}

func getMachineDeploymentCondition(conditions []machinev1alpha1.MachineDeploymentCondition, conditionType machinev1alpha1.MachineDeploymentConditionType) *machinev1alpha1.MachineDeploymentCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}
