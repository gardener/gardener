// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"net/http"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"k8s.io/client-go/rest"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
)

func requiredConditionMissing(conditionType string) error {
	return fmt.Errorf("condition %q is missing", conditionType)
}

func checkConditionState(conditionType string, expected, actual, reason, message string) error {
	if expected != actual {
		return fmt.Errorf("condition %q has invalid status %s (expected %s) due to %s: %s",
			conditionType, actual, expected, reason, message)
	}
	return nil
}

func getDeploymentCondition(conditions []appsv1.DeploymentCondition, conditionType appsv1.DeploymentConditionType) *appsv1.DeploymentCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
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

func getNodeCondition(conditions []corev1.NodeCondition, conditionType corev1.NodeConditionType) *corev1.NodeCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

var (
	trueDeploymentConditionTypes = []appsv1.DeploymentConditionType{
		appsv1.DeploymentAvailable,
	}

	trueOptionalDeploymentConditionTypes = []appsv1.DeploymentConditionType{
		appsv1.DeploymentProgressing,
	}

	falseDeploymentConditionTypes = []appsv1.DeploymentConditionType{
		appsv1.DeploymentReplicaFailure,
	}
)

// CheckDeployment checks whether the given Deployment is healthy.
// A deployment is considered healthy if the controller observed its current revision and
// if the number of updated replicas is equal to the number of replicas.
func CheckDeployment(deployment *appsv1.Deployment) error {
	if deployment.Status.ObservedGeneration < deployment.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", deployment.Status.ObservedGeneration, deployment.Generation)
	}

	for _, trueConditionType := range trueDeploymentConditionTypes {
		conditionType := string(trueConditionType)
		condition := getDeploymentCondition(deployment.Status.Conditions, trueConditionType)
		if condition == nil {
			return requiredConditionMissing(conditionType)
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	for _, trueOptionalConditionType := range trueOptionalDeploymentConditionTypes {
		conditionType := string(trueOptionalConditionType)
		condition := getDeploymentCondition(deployment.Status.Conditions, trueOptionalConditionType)
		if condition == nil {
			continue
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	for _, falseConditionType := range falseDeploymentConditionTypes {
		conditionType := string(falseConditionType)
		condition := getDeploymentCondition(deployment.Status.Conditions, falseConditionType)
		if condition == nil {
			continue
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionFalse), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
}

// CheckStatefulSet checks whether the given StatefulSet is healthy.
// A StatefulSet is considered healthy if its controller observed its current revision,
// it is not in an update (i.e. UpdateRevision is empty) and if its current replicas are equal to
// its desired replicas.
func CheckStatefulSet(statefulSet *appsv1.StatefulSet) error {
	if statefulSet.Status.ObservedGeneration < statefulSet.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", statefulSet.Status.ObservedGeneration, statefulSet.Generation)
	}

	replicas := int32(1)
	if statefulSet.Spec.Replicas != nil {
		replicas = *statefulSet.Spec.Replicas
	}

	if statefulSet.Status.ReadyReplicas < replicas {
		return fmt.Errorf("not enough ready replicas (%d/%d)", statefulSet.Status.ReadyReplicas, replicas)
	}
	return nil
}

func daemonSetMaxUnavailable(daemonSet *appsv1.DaemonSet) int32 {
	if daemonSet.Status.DesiredNumberScheduled == 0 || daemonSet.Spec.UpdateStrategy.Type != appsv1.RollingUpdateDaemonSetStrategyType {
		return 0
	}

	rollingUpdate := daemonSet.Spec.UpdateStrategy.RollingUpdate
	if rollingUpdate == nil {
		return 0
	}

	maxUnavailable, err := intstr.GetValueFromIntOrPercent(rollingUpdate.MaxUnavailable, int(daemonSet.Status.DesiredNumberScheduled), false)
	if err != nil {
		return 0
	}

	return int32(maxUnavailable)
}

// CheckDaemonSet checks whether the given DaemonSet is healthy.
// A DaemonSet is considered healthy if its controller observed its current revision and if
// its desired number of scheduled pods is equal to its updated number of scheduled pods.
func CheckDaemonSet(daemonSet *appsv1.DaemonSet) error {
	if daemonSet.Status.ObservedGeneration < daemonSet.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", daemonSet.Status.ObservedGeneration, daemonSet.Generation)
	}

	maxUnavailable := daemonSetMaxUnavailable(daemonSet)

	if requiredAvailable := daemonSet.Status.DesiredNumberScheduled - maxUnavailable; daemonSet.Status.CurrentNumberScheduled < requiredAvailable {
		return fmt.Errorf("not enough available replicas (%d/%d)", daemonSet.Status.CurrentNumberScheduled, requiredAvailable)
	}
	return nil
}

var (
	trueNodeConditionTypes = []corev1.NodeConditionType{
		corev1.NodeReady,
	}

	falseNodeConditionTypes = []corev1.NodeConditionType{
		corev1.NodeDiskPressure,
		corev1.NodeMemoryPressure,
		corev1.NodeNetworkUnavailable,
		corev1.NodeOutOfDisk,
		corev1.NodePIDPressure,
	}
)

// CheckNode checks whether the given Node is healthy.
// A node is considered healthy if it has a `corev1.NodeReady` condition and this condition reports
// `corev1.ConditionTrue`.
func CheckNode(node *corev1.Node) error {
	for _, trueConditionType := range trueNodeConditionTypes {
		conditionType := string(trueConditionType)
		condition := getNodeCondition(node.Status.Conditions, trueConditionType)
		if condition == nil {
			return requiredConditionMissing(conditionType)
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	for _, falseConditionType := range falseNodeConditionTypes {
		conditionType := string(falseConditionType)
		condition := getNodeCondition(node.Status.Conditions, falseConditionType)
		if condition == nil {
			continue
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionFalse), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
}

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
		if err := checkConditionState(conditionType, string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	for _, trueOptionalConditionType := range trueOptionalMachineDeploymentConditionTypes {
		conditionType := string(trueOptionalConditionType)
		condition := getMachineDeploymentCondition(deployment.Status.Conditions, trueOptionalConditionType)
		if condition == nil {
			continue
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	for _, falseConditionType := range falseMachineDeploymentConditionTypes {
		conditionType := string(falseConditionType)
		condition := getMachineDeploymentCondition(deployment.Status.Conditions, falseConditionType)
		if condition == nil {
			continue
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionFalse), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
}

// Now determines the current time.
var Now = time.Now

// ConditionerFunc to update a condition with type and message
type conditionerFunc func(conditionType string, message string) gardencorev1alpha1.Condition

// CheckAPIServerAvailability checks if the API server of a cluster is reachable and measure the response time.
func CheckAPIServerAvailability(condition gardencorev1alpha1.Condition, restClient rest.Interface, conditioner conditionerFunc) gardencorev1alpha1.Condition {
	now := Now()
	response := restClient.Get().AbsPath("/healthz").Do()
	responseDurationText := fmt.Sprintf("[response_time:%dms]", Now().Sub(now).Nanoseconds()/time.Millisecond.Nanoseconds())
	if response.Error() != nil {
		message := fmt.Sprintf("Request to API server /healthz endpoint failed. %s (%s)", responseDurationText, response.Error().Error())
		return conditioner("HealthzRequestFailed", message)
	}

	// Determine the status code of the response.
	var statusCode int
	response.StatusCode(&statusCode)

	if statusCode != http.StatusOK {
		var body string
		bodyRaw, err := response.Raw()
		if err != nil {
			body = fmt.Sprintf("Could not parse response body: %s", err.Error())
		} else {
			body = string(bodyRaw)
		}
		message := fmt.Sprintf("API server /healthz endpoint endpoint check returned a non ok status code %d. %s (%s)", statusCode, responseDurationText, body)
		return conditioner("HealthzRequestError", message)
	}

	message := fmt.Sprintf("API server /healthz endpoint responded with success status code. %s", responseDurationText)
	return helper.UpdatedCondition(condition, gardencorev1alpha1.ConditionTrue, "HealthzRequestFailed", message)
}
