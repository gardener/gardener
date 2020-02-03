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

package health

import (
	"fmt"

	"github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1/helper"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// CheckManagedResource checks if all conditions of a ManagedResource ('ResourcesApplied' and 'ResourcesHealthy')
// are True and .status.observedGeneration matches the current .metadata.generation
func CheckManagedResource(mr *v1alpha1.ManagedResource) error {
	if err := CheckManagedResourceApplied(mr); err != nil {
		return err
	}
	if err := CheckManagedResourceHealthy(mr); err != nil {
		return err
	}

	return nil
}

// CheckManagedResourceApplied checks if the condition 'ResourcesApplied' of a ManagedResource
// is True and the .status.observedGeneration matches the current .metadata.generation
func CheckManagedResourceApplied(mr *v1alpha1.ManagedResource) error {
	status := mr.Status
	if status.ObservedGeneration != mr.GetGeneration() {
		return fmt.Errorf("observed generation of managed resource %s/%s outdated (%d/%d)", mr.GetNamespace(), mr.GetName(), status.ObservedGeneration, mr.GetGeneration())
	}

	conditionApplied := helper.GetCondition(status.Conditions, v1alpha1.ResourcesApplied)

	if conditionApplied == nil {
		return fmt.Errorf("condition %s for managed resource %s/%s has not been reported yet", v1alpha1.ResourcesApplied, mr.GetNamespace(), mr.GetName())
	} else if conditionApplied.Status != v1alpha1.ConditionTrue {
		return fmt.Errorf("condition %s of managed resource %s/%s is %s: %s", v1alpha1.ResourcesApplied, mr.GetNamespace(), mr.GetName(), conditionApplied.Status, conditionApplied.Message)
	}

	return nil
}

// CheckManagedResourceHealthy checks if the condition 'ResourcesHealthy' of a ManagedResource is True
func CheckManagedResourceHealthy(mr *v1alpha1.ManagedResource) error {
	status := mr.Status
	conditionHealthy := helper.GetCondition(status.Conditions, v1alpha1.ResourcesHealthy)

	if conditionHealthy == nil {
		return fmt.Errorf("condition %s for managed resource %s/%s has not been reported yet", v1alpha1.ResourcesHealthy, mr.GetNamespace(), mr.GetName())
	} else if conditionHealthy.Status != v1alpha1.ConditionTrue {
		return fmt.Errorf("condition %s of managed resource %s/%s is %s: %s", v1alpha1.ResourcesHealthy, mr.GetNamespace(), mr.GetName(), conditionHealthy.Status, conditionHealthy.Message)
	}

	return nil
}

var (
	trueCrdConditionTypes = []apiextensionsv1beta1.CustomResourceDefinitionConditionType{
		apiextensionsv1beta1.NamesAccepted, apiextensionsv1beta1.Established,
	}
	falseOptionalCrdConditionTypes = []apiextensionsv1beta1.CustomResourceDefinitionConditionType{
		apiextensionsv1beta1.Terminating,
	}
)

// CheckCustomResourceDefinition checks whether the given CustomResourceDefinition is healthy.
// A CRD is considered healthy if its `NamesAccepted` and `Established` conditions are with status `True`
// and its `Terminating` condition is missing or has status `False`.
func CheckCustomResourceDefinition(crd *apiextensionsv1beta1.CustomResourceDefinition) error {
	for _, trueConditionType := range trueCrdConditionTypes {
		conditionType := string(trueConditionType)
		condition := getCustomResourceDefinitionCondition(crd.Status.Conditions, trueConditionType)
		if condition == nil {
			return requiredConditionMissing(conditionType)
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	for _, falseOptionalConditionType := range falseOptionalCrdConditionTypes {
		conditionType := string(falseOptionalConditionType)
		condition := getCustomResourceDefinitionCondition(crd.Status.Conditions, falseOptionalConditionType)
		if condition == nil {
			continue
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionFalse), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
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
	trueDeploymentConditionTypes = []appsv1.DeploymentConditionType{
		appsv1.DeploymentAvailable,
	}

	trueOptionalDeploymentConditionTypes = []appsv1.DeploymentConditionType{
		appsv1.DeploymentProgressing,
	}

	falseOptionalDeploymentConditionTypes = []appsv1.DeploymentConditionType{
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

	for _, falseOptionalConditionType := range falseOptionalDeploymentConditionTypes {
		conditionType := string(falseOptionalConditionType)
		condition := getDeploymentCondition(deployment.Status.Conditions, falseOptionalConditionType)
		if condition == nil {
			continue
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionFalse), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
}

// CheckJob checks whether the given Job is healthy.
// A Job is considered healthy if its `JobFailed` condition is missing or has status `False`.
func CheckJob(job *batchv1.Job) error {
	condition := getJobCondition(job.Status.Conditions, batchv1.JobFailed)
	if condition == nil {
		return nil
	}
	if err := checkConditionState(string(batchv1.JobFailed), string(corev1.ConditionFalse), string(condition.Status), condition.Reason, condition.Message); err != nil {
		return err
	}

	return nil
}

var (
	healthyPodPhases = []corev1.PodPhase{
		corev1.PodRunning, corev1.PodSucceeded,
	}
)

// CheckPod checks whether the given Pod is healthy.
// A Pod is considered healthy if its `.status.phase` is `Running` or `Succeeded`.
func CheckPod(pod *corev1.Pod) error {
	var phase = pod.Status.Phase
	for _, healthyPhase := range healthyPodPhases {
		if phase == healthyPhase {
			return nil
		}
	}

	return fmt.Errorf("pod is in invalid phase %q (expected one of %q)",
		phase, healthyPodPhases)
}

// CheckReplicaSet checks whether the given ReplicaSet is healthy.
// A ReplicaSet is considered healthy if the controller observed its current revision and
// if the number of ready replicas is equal to the number of replicas.
func CheckReplicaSet(rs *appsv1.ReplicaSet) error {
	if rs.Status.ObservedGeneration < rs.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", rs.Status.ObservedGeneration, rs.Generation)
	}

	var replicas = rs.Spec.Replicas
	if replicas != nil && rs.Status.ReadyReplicas < *replicas {
		return fmt.Errorf("ReplicaSet does not have minimum availability")
	}

	return nil
}

// CheckReplicationController check whether the given ReplicationController is healthy.
// A ReplicationController is considered healthy if the controller observed its current revision and
// if the number of ready replicas is equal to the number of replicas.
func CheckReplicationController(rc *corev1.ReplicationController) error {
	if rc.Status.ObservedGeneration < rc.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", rc.Status.ObservedGeneration, rc.Generation)
	}

	var replicas = rc.Spec.Replicas
	if replicas != nil && rc.Status.ReadyReplicas < *replicas {
		return fmt.Errorf("ReplicationController does not have minimum availability")
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

func getCustomResourceDefinitionCondition(conditions []apiextensionsv1beta1.CustomResourceDefinitionCondition, conditionType apiextensionsv1beta1.CustomResourceDefinitionConditionType) *apiextensionsv1beta1.CustomResourceDefinitionCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
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

func getJobCondition(conditions []batchv1.JobCondition, conditionType batchv1.JobConditionType) *batchv1.JobCondition {
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

func checkConditionState(conditionType string, expected, actual, reason, message string) error {
	if expected != actual {
		return fmt.Errorf("condition %q has invalid status %s (expected %s) due to %s: %s",
			conditionType, actual, expected, reason, message)
	}
	return nil
}
