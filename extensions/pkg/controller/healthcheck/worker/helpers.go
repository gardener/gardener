// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package worker

import (
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck/general"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
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

func checkSufficientNodesAvailable(nodeList *corev1.NodeList, machineDeploymentList *machinev1alpha1.MachineDeploymentList) (bool, *string, error) {
	desiredMachines := getDesiredMachineCount(machineDeploymentList.Items)

	if registeredNodes := len(nodeList.Items); registeredNodes < desiredMachines {
		reason := "MissingNodes"
		err := fmt.Errorf("not enough worker nodes registered in the cluster (%d/%d)", registeredNodes, desiredMachines)
		return false, &reason, err
	}
	return true, nil, nil
}

func getDesiredMachineCount(machineDeploymentList []machinev1alpha1.MachineDeployment) int {
	desiredMachines := 0
	for _, machineDeployment := range machineDeploymentList {
		if machineDeployment.DeletionTimestamp == nil {
			desiredMachines += int(machineDeployment.Spec.Replicas)
		}
	}
	return desiredMachines
}

func machineDeploymentsAreHealthy(machineDeployments []machinev1alpha1.MachineDeployment) (bool, *string, error) {
	for _, deployment := range machineDeployments {
		if err := CheckMachineDeployment(&deployment); err != nil {
			reason := "MachineDeploymentUnhealthy"
			err := fmt.Errorf("machine deployment %s in namespace %s is unhealthy: %v", deployment.Name, deployment.Namespace, err)
			return false, &reason, err
		}
	}
	return true, nil, nil
}

// CheckMachineDeployment checks whether the given MachineDeployment is healthy.
// A MachineDeployment is considered healthy if its controller observed its current revision and if
// its desired number of replicas is equal to its updated replicas.
func CheckMachineDeployment(deployment *machinev1alpha1.MachineDeployment) error {
	if deployment.Status.ObservedGeneration < deployment.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", deployment.Status.ObservedGeneration, deployment.Generation)
	}

	for _, trueConditionType := range trueMachineDeploymentConditionTypes {
		conditionType := string(trueConditionType)
		condition := getMachineDeploymentCondition(deployment.Status.Conditions, trueConditionType)
		if condition == nil {
			return requiredConditionMissing(conditionType)
		}
		if err := general.CheckConditionState(conditionType, string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	for _, trueOptionalConditionType := range trueOptionalMachineDeploymentConditionTypes {
		conditionType := string(trueOptionalConditionType)
		condition := getMachineDeploymentCondition(deployment.Status.Conditions, trueOptionalConditionType)
		if condition == nil {
			continue
		}
		if err := general.CheckConditionState(conditionType, string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	for _, falseConditionType := range falseMachineDeploymentConditionTypes {
		conditionType := string(falseConditionType)
		condition := getMachineDeploymentCondition(deployment.Status.Conditions, falseConditionType)
		if condition == nil {
			continue
		}
		if err := general.CheckConditionState(conditionType, string(corev1.ConditionFalse), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
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

func requiredConditionMissing(conditionType string) error {
	return fmt.Errorf("condition %q is missing", conditionType)
}
