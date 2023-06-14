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
