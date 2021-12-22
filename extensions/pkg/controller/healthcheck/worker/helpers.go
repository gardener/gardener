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
	"math"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// NowFunc is a function returning the current time.
	// Exposed for testing.
	NowFunc = time.Now
)

// CheckMachineDeployment checks whether the given MachineDeployment is healthy.
// A MachineDeployment is considered healthy if its controller observed its current revision and if
// its desired number of replicas is equal to its updated replicas.
func CheckMachineDeployment(deployment *machinev1alpha1.MachineDeployment) error {
	if deployment.Status.ObservedGeneration < deployment.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", deployment.Status.ObservedGeneration, deployment.Generation)
	}

	for _, trueConditionType := range trueMachineDeploymentConditionTypes {
		condition := getMachineDeploymentCondition(deployment.Status.Conditions, trueConditionType)
		if condition == nil {
			return requiredConditionMissing(trueConditionType)
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

func checkMachineDeploymentsHealthy(machineDeployments []machinev1alpha1.MachineDeployment) (bool, error) {
	for _, deployment := range machineDeployments {
		for _, failedMachine := range deployment.Status.FailedMachines {
			return false, fmt.Errorf("machine %q failed: %s", failedMachine.Name, failedMachine.LastOperation.Description)
		}

		if err := CheckMachineDeployment(&deployment); err != nil {
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

	var pendingMachines, erroneousMachines, terminatingMachines int
	for _, machine := range machineList.Items {
		switch machine.Status.CurrentStatus.Phase {
		case machinev1alpha1.MachineRunning, machinev1alpha1.MachineAvailable:
			// machine is already running fine
			continue
		case machinev1alpha1.MachineTerminating:
			// terminating machines are not handled by this check. This is to avoid showing terminating/draining machines during a rolling update as erroneous.
			terminatingMachines++
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
		return gardencorev1beta1.ConditionFalse, fmt.Errorf("%s erroneous", cosmeticNodeMessage(erroneousMachines))
	}
	if pendingMachines == 0 {
		return gardencorev1beta1.ConditionFalse, fmt.Errorf("not enough ready worker nodes registered in the cluster (%d/%d)", readyNodes, desiredMachines)
	}

	return gardencorev1beta1.ConditionProgressing, fmt.Errorf("%s provisioning and should join the cluster soon", cosmeticNodeMessage(pendingMachines))
}

func checkNodesScalingDown(machineList *machinev1alpha1.MachineList, nodeList *corev1.NodeList, registeredNodes, desiredMachines int, machineDrainTimeoutThreshold *int) (gardencorev1beta1.ConditionStatus, *time.Duration, error) {
	if registeredNodes == desiredMachines {
		return gardencorev1beta1.ConditionTrue, nil, nil
	}

	// Check if all nodes that are cordoned map to machines with a deletion timestamp. This might be the case during
	// a rolling update.
	nodeNameToMachine := map[string]machinev1alpha1.Machine{}
	for _, machine := range machineList.Items {
		if machine.Labels != nil && machine.Labels["node"] != "" {
			nodeNameToMachine[machine.Labels["node"]] = machine
		}
	}

	// NOTE: The drain window is defined as the machineDrainTimeout on the Worker Pool * machineDrainTimeoutThreshold (e.g 0.8)
	// The machineDrainTimeoutThreshold is specific to the extension
	var (
		// Machines of worker pool not defining a machineDrainTimeout
		cordonedNodesWithoutCustomDrainTimeout int
		// Machines not exceeding the worker pool's machineDrainTimeout.
		cordonedNodesWithinCustomDrainWindow int
		// Machines that failed to be drained during the drain window. This causes the health check to fail.
		// The machines are drained as soon as the whole duration defined in the worker pool's machineDrainTimeout is expired.
		cordonedNodesExceedingCustomDrainWindow int
		// Threshold that marks this health check as progressing
		scaleDownProgressingThresholdNotExceeded = defaultScaleDownProgressingThreshold
		// Threshold that marks this health check as failed. The overall health check in the Shoot might still be shown as progressing.
		scaleDownProgressingThresholdExceeded = defaultScaleDownProgressingThreshold

		minDurationUntilDrainWindowExpires        time.Duration
		nameNextWorkerPoolWithExpiringDrainWindow string

		nameWorkerPoolWithAlreadyExpiringDrainWindow string
		nextForceDeletionTime                        time.Time
		minDurationTillForceDeletion                 time.Duration
	)

	for _, node := range nodeList.Items {
		if node.Spec.Unschedulable {
			machine, ok := nodeNameToMachine[node.Name]
			if !ok {
				return gardencorev1beta1.ConditionFalse, nil, fmt.Errorf("machine object for cordoned node %q not found", node.Name)
			}
			if machine.DeletionTimestamp == nil {
				return gardencorev1beta1.ConditionFalse, nil, fmt.Errorf("cordoned node %q found but corresponding machine object does not have a deletion timestamp", node.Name)
			}

			// Only count as a condoned node when exceeds drain threshold
			// this check is done here and not on a higher level, as it is a relative value based on
			// the machine's individual drain timeout
			if machine.Spec.MachineConfiguration != nil &&
				machine.Spec.MachineConfiguration.MachineDrainTimeout != nil &&
				machineDrainTimeoutThreshold != nil {
				exceeded, progressingThresholdBasedOnMachineDrainTimeout, durationTillExpiration := ThresholdIsExceeded(machine.DeletionTimestamp, *machine.Spec.MachineDrainTimeout, *machineDrainTimeoutThreshold)

				if !exceeded {
					cordonedNodesWithinCustomDrainWindow++

					// Remember the highest threshold. This is for the case that no nodes exceed the machineDrainTimeoutThreshold!
					// Reason: we know in that case that all cordoned machines respect the threshold for the custom MachineDrainTimeout => we have to return a threshold that marks this check as progressing, not failed.
					// If a scaleDownProgressingThreshold lower than the highest threshold for any machine is returned, it causes a failing health check because the
					// LastTransitionTime on the ExtensionResource for the EveryNodeReady condition is older.
					if progressingThresholdBasedOnMachineDrainTimeout > scaleDownProgressingThresholdNotExceeded {
						scaleDownProgressingThresholdNotExceeded = progressingThresholdBasedOnMachineDrainTimeout
					}

					// to have a useful error message on the extension resource, also remember the time until the next drain window expires
					if minDurationUntilDrainWindowExpires == 0 || durationTillExpiration < minDurationUntilDrainWindowExpires {
						minDurationUntilDrainWindowExpires = durationTillExpiration
						nameNextWorkerPoolWithExpiringDrainWindow = machine.Spec.NodeTemplateSpec.Labels[gardencorev1beta1constants.LabelWorkerPool]
					}

					continue
				}

				// Remember the lowest threshold. This is for the case that at least the machine of one node exceeds the machineDrainTimeoutThreshold!
				// we have to return a threshold that marks this check as failed, not progressing.
				// This also overwrites the defaultScaleDownProgressingThreshold if the MachineDrainTimeout of the current machine is lower than that
				// (otherwise, we might never see an error for a machine whose timeout is lower than the defaultScaleDownProgressingThreshold).
				if progressingThresholdBasedOnMachineDrainTimeout < scaleDownProgressingThresholdExceeded {
					scaleDownProgressingThresholdExceeded = progressingThresholdBasedOnMachineDrainTimeout
				}

				// calculate the time when the MCM will force drain the machine
				// this is important information for the Shoot owner and operator and should be visible in the Shoot's readiness conditions
				forceDeletionTime, durationTillForceDeletion := GetDurationTillForceDeletion(machine.DeletionTimestamp, *machine.Spec.MachineDrainTimeout)

				if minDurationTillForceDeletion == 0 || durationTillForceDeletion < minDurationTillForceDeletion {
					minDurationTillForceDeletion = durationTillForceDeletion
					nextForceDeletionTime = forceDeletionTime
					nameWorkerPoolWithAlreadyExpiringDrainWindow = machine.Spec.NodeTemplateSpec.Labels[gardencorev1beta1constants.LabelWorkerPool]
				}

				cordonedNodesExceedingCustomDrainWindow++
				continue
			}

			cordonedNodesWithoutCustomDrainTimeout++
		}
	}

	// All good: there are no nodes without a machineDrainTimeout and there are no cordoned nodes whose machines exceed the machineDrainTimeoutThreshold.
	// Return a threshold that does not mark this test as failed.
	if cordonedNodesWithoutCustomDrainTimeout == 0 && cordonedNodesExceedingCustomDrainWindow == 0 && cordonedNodesWithinCustomDrainWindow > 0 {
		return gardencorev1beta1.ConditionProgressing, &scaleDownProgressingThresholdNotExceeded, fmt.Errorf("%s waiting to be drained from pods. The %s within the regular drain window. The Shoot's next drain window targets worker pool %q and ends in %s", cosmeticNodeMessage(cordonedNodesWithinCustomDrainWindow), cosmeticNodeMessageNoCount(cordonedNodesWithinCustomDrainWindow), nameNextWorkerPoolWithExpiringDrainWindow, minDurationUntilDrainWindowExpires.Round(time.Second).String())
	}

	// There are no nodes without a machineDrainTimeout but there are cordoned nodes whose machines exceed the MachineDrainTimeout.
	if cordonedNodesWithoutCustomDrainTimeout == 0 && cordonedNodesExceedingCustomDrainWindow > 0 {
		if time.Now().UTC().Before(nextForceDeletionTime) {
			return gardencorev1beta1.ConditionFalse, nil, fmt.Errorf("%s failed to be drained from pods within the regular drain window. Please note, that nodes of worker pool %q are forcefully terminated at %s (in %s)", cosmeticNodeSingularPlural(cordonedNodesExceedingCustomDrainWindow), nameWorkerPoolWithAlreadyExpiringDrainWindow, nextForceDeletionTime.Round(time.Second).String(), minDurationTillForceDeletion.Round(time.Second).String())
		}
		return gardencorev1beta1.ConditionFalse, nil, fmt.Errorf("%s failed to be drained from pods within the regular drain window. Please note, that nodes of worker pool %q are forcefully terminated since %s", cosmeticNodeSingularPlural(cordonedNodesExceedingCustomDrainWindow), nameWorkerPoolWithAlreadyExpiringDrainWindow, nextForceDeletionTime.Round(time.Second).String())
	}

	totalCordonedNodes := cordonedNodesWithoutCustomDrainTimeout + cordonedNodesExceedingCustomDrainWindow + cordonedNodesWithinCustomDrainWindow

	// There are too many registered node, but no machine is currently being drained.
	if totalCordonedNodes == 0 && registeredNodes > desiredMachines {
		return gardencorev1beta1.ConditionFalse, nil, fmt.Errorf("too many worker nodes are registered. Exceeding maximum desired machine count (%d/%d)", registeredNodes, desiredMachines)
	}

	// If there are still more nodes than desired then report an error.
	if registeredNodes-totalCordonedNodes > desiredMachines {
		return gardencorev1beta1.ConditionFalse, nil, fmt.Errorf("too many worker nodes are registered. Exceeding maximum desired machine count (%d/%d). Additionally, %s waiting to be completely drained from pods", registeredNodes, desiredMachines, cosmeticNodeMessage(totalCordonedNodes))
	}

	// Either no machineDrainTimeoutThreshold was defined (Status: progressing or failed), or the machineDrainTimeoutThreshold of at least one machine is exceeded (Status: failed)
	return gardencorev1beta1.ConditionProgressing, &scaleDownProgressingThresholdExceeded, fmt.Errorf("%s waiting to be drained from pods. If this persists, check your pod disruption budgets and pending finalizers. Please note, that nodes which fail to be drained are forcefully terminated", cosmeticNodeMessage(totalCordonedNodes))
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

func requiredConditionMissing(conditionType machinev1alpha1.MachineDeploymentConditionType) error {
	return fmt.Errorf("condition %q is missing", conditionType)
}

func cosmeticNodeMessage(numberOfNodes int) string {
	if numberOfNodes == 1 {
		return fmt.Sprintf("%d node is", numberOfNodes)
	}
	return fmt.Sprintf("%d nodes are", numberOfNodes)
}

func cosmeticNodeSingularPlural(numberOfNodes int) string {
	if numberOfNodes == 1 {
		return fmt.Sprintf("%d node", numberOfNodes)
	}
	return fmt.Sprintf("%d nodes", numberOfNodes)
}

func cosmeticNodeMessageNoCount(numberOfNodes int) string {
	if numberOfNodes == 1 {
		return "node is"
	}
	return "nodes are"
}

// ThresholdIsExceeded returns
// - true in case the given thresholdPercentage is exceeded.
// - the threshold based on the given thresholdPercentage.
// - the duration until the threshold is exceeded, or the duration the threshold is already exceeded
func ThresholdIsExceeded(base *metav1.Time, duration metav1.Duration, thresholdPercentage int) (bool, time.Duration, time.Duration) {
	elapsedValidity := NowFunc().UTC().Sub(base.Time.UTC()).Seconds()
	var validityThreshold = duration.Seconds() * (float64(thresholdPercentage) / 100)

	return elapsedValidity > validityThreshold, time.Duration(int64(validityThreshold)) * time.Second, time.Duration(int64(math.Abs(elapsedValidity-validityThreshold))) * time.Second
}

// GetDurationTillForceDeletion calculates the time when a machine is force deleted by the MCM given the deletion timestamp
// of the machine and the machineDrainTimeout
// returns
// - the UTC time when the force deletion takes place
// - the time until force deletion
func GetDurationTillForceDeletion(machineDeletionTimestamp *metav1.Time, drainTimeout metav1.Duration) (time.Time, time.Duration) {
	forceDeletionTime := machineDeletionTimestamp.Time.UTC().Add(drainTimeout.Duration)
	return forceDeletionTime, forceDeletionTime.Sub(time.Now().UTC())
}
