// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

func checkMachineDeploymentsHealthy(machineDeployments []machinev1alpha1.MachineDeployment) (bool, error) {
	for _, deployment := range machineDeployments {
		for _, failedMachine := range deployment.Status.FailedMachines {
			return false, fmt.Errorf("machine %q failed: %s", failedMachine.Name, failedMachine.LastOperation.Description)
		}

		if err := health.CheckMachineDeployment(&deployment); err != nil {
			return false, fmt.Errorf("machine deployment %q in namespace %q is unhealthy: %w", deployment.Name, deployment.Namespace, err)
		}
	}

	return true, nil
}

func checkNodesScalingUp(machineList *machinev1alpha1.MachineList, readyNodes, desiredMachines int) (gardencorev1beta1.ConditionStatus, error) {
	if readyNodes == desiredMachines {
		return gardencorev1beta1.ConditionTrue, nil
	}

	if machineObjects := len(machineList.Items); machineObjects < desiredMachines {
		return gardencorev1beta1.ConditionFalse, fmt.Errorf("not enough machine objects created yet (%d/%d)", machineObjects, desiredMachines)
	}

	var pendingMachines, erroneousMachines int
	for _, machine := range machineList.Items {
		switch machine.Status.CurrentStatus.Phase {
		case machinev1alpha1.MachineRunning, machinev1alpha1.MachineAvailable:
			// machine is already running fine
			continue
		case machinev1alpha1.MachinePending, "": // https://github.com/gardener/machine-controller-manager/issues/466
			// machine is in the process of being created
			pendingMachines++
		default:
			// undesired machine phase
			erroneousMachines++
		}
	}

	if erroneousMachines > 0 {
		return gardencorev1beta1.ConditionFalse, fmt.Errorf("%s erroneous", cosmeticMachineMessage(erroneousMachines))
	}
	if pendingMachines == 0 {
		return gardencorev1beta1.ConditionFalse, fmt.Errorf("not enough ready worker nodes registered in the cluster (%d/%d)", readyNodes, desiredMachines)
	}

	return gardencorev1beta1.ConditionProgressing, fmt.Errorf("%s provisioning and should join the cluster soon", cosmeticMachineMessage(pendingMachines))
}

func checkNodesScalingDown(machineList *machinev1alpha1.MachineList, nodeList *corev1.NodeList, registeredNodes, desiredMachines int) (gardencorev1beta1.ConditionStatus, error) {
	if registeredNodes == desiredMachines {
		return gardencorev1beta1.ConditionTrue, nil
	}

	// Check if all nodes that are cordoned map to machines with a deletion timestamp. This might be the case during
	// a rolling update.
	nodeNameToMachine := map[string]machinev1alpha1.Machine{}
	for _, machine := range machineList.Items {
		if machine.Labels != nil && machine.Labels["node"] != "" {
			nodeNameToMachine[machine.Labels["node"]] = machine
		}
	}

	var cordonedNodes int
	for _, node := range nodeList.Items {
		if metav1.HasAnnotation(node.ObjectMeta, AnnotationKeyNotManagedByMCM) && node.Annotations[AnnotationKeyNotManagedByMCM] == "1" {
			continue
		}
		if node.Spec.Unschedulable {
			machine, ok := nodeNameToMachine[node.Name]
			if !ok {
				return gardencorev1beta1.ConditionFalse, fmt.Errorf("machine object for cordoned node %q not found", node.Name)
			}
			if machine.DeletionTimestamp == nil {
				return gardencorev1beta1.ConditionFalse, fmt.Errorf("cordoned node %q found but corresponding machine object does not have a deletion timestamp", node.Name)
			}
			cordonedNodes++
		}
	}

	// If there are still more nodes than desired then report an error.
	if registeredNodes-cordonedNodes != desiredMachines {
		return gardencorev1beta1.ConditionFalse, fmt.Errorf("too many worker nodes are registered. Exceeding maximum desired machine count (%d/%d)", registeredNodes, desiredMachines)
	}

	return gardencorev1beta1.ConditionProgressing, fmt.Errorf("%s waiting to be completely drained from pods. If this persists, check your pod disruption budgets and pending finalizers. Please note, that nodes that fail to be drained will be deleted automatically", cosmeticMachineMessage(cordonedNodes))
}

func getDesiredMachineCount(machineDeployments []machinev1alpha1.MachineDeployment) int {
	desiredMachines := 0
	for _, deployment := range machineDeployments {
		if deployment.DeletionTimestamp == nil {
			desiredMachines += int(deployment.Spec.Replicas)
		}
	}
	return desiredMachines
}

func cosmeticMachineMessage(numberOfMachines int) string {
	if numberOfMachines == 1 {
		return fmt.Sprintf("%d machine is", numberOfMachines)
	}
	return fmt.Sprintf("%d machines are", numberOfMachines)
}
