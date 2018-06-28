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

package botanist

import (
	"fmt"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CheckConditionControlPlaneHealthy checks whether the control plane of the Shoot cluster is healthy,
// i.e. whether all containers running in the relevant namespace in the Seed cluster are healthy.
func (b *Botanist) CheckConditionControlPlaneHealthy(condition *gardenv1beta1.Condition) *gardenv1beta1.Condition {
	response, err := b.K8sShootClient.Curl("healthz")
	if err != nil {
		return helper.ModifyCondition(condition, corev1.ConditionFalse, "KubeAPIServerNotHealthy", fmt.Sprintf("Could not reach Shoot cluster kube-apiserver's /healthz endpoint: '%s'", err.Error()))
	}
	var statusCode int
	response.StatusCode(&statusCode)
	if statusCode < 200 || statusCode >= 400 {
		return helper.ModifyCondition(condition, corev1.ConditionFalse, "KubeAPIServerNotHealthy", "Shoot cluster kube-apiserver's /healthz endpoint indicates unhealthiness.")
	}

	// Check whether the number of availableReplicas matches the number of desired replicas for all deployments.
	if updatedCondition, modified := verifyDeploymentHealthiness(condition, b.K8sSeedClient, b.Shoot.SeedNamespace, metav1.ListOptions{}); modified {
		return updatedCondition
	}

	// Check whether the number of running containers matches the number of actual containers within the pods (i.e., everything is running).
	if updatedCondition, modified := verifyPodHealthiness(condition, b.K8sSeedClient, b.Shoot.SeedNamespace, metav1.ListOptions{}); modified {
		return updatedCondition
	}

	return helper.ModifyCondition(condition, corev1.ConditionTrue, "AllPodsInRunningState", "All pods running the Shoot namespace in the Seed cluster are healthy.")
}

// CheckConditionEveryNodeReady checks whether every node registered at the Shoot cluster is in "Ready" state, that
// as many nodes are registered as desired, and that every machine is running.
func (b *Botanist) CheckConditionEveryNodeReady(condition *gardenv1beta1.Condition) *gardenv1beta1.Condition {
	// Check whether every Node registered to the API server is ready.
	nodeList, err := b.K8sShootClient.ListNodes(metav1.ListOptions{})
	if err != nil {
		return helper.ModifyCondition(condition, corev1.ConditionUnknown, "FetchNodeListFailed", err.Error())
	}
	for _, node := range nodeList.Items {
		if !node.Spec.Unschedulable {
			for _, nodeCondition := range node.Status.Conditions {
				if nodeCondition.Type == corev1.NodeReady && nodeCondition.Status != corev1.ConditionTrue {
					return helper.ModifyCondition(condition, corev1.ConditionFalse, "NodeNotReady", fmt.Sprintf("Node %s is not ready.", node.Name))
				}
			}
		}
	}

	// Check whether at least as many machines as desired are registered to the cluster.
	var (
		registeredMachine = len(nodeList.Items)
		desiredMachines   int32
	)

	machineDeploymentList, err := b.K8sSeedClient.MachineClientset().MachineV1alpha1().MachineDeployments(b.Shoot.SeedNamespace).List(metav1.ListOptions{})
	if err != nil {
		return helper.ModifyCondition(condition, corev1.ConditionUnknown, "FetchMachineDeploymentListFailed", err.Error())
	}
	for _, machineDeployment := range machineDeploymentList.Items {
		if machineDeployment.DeletionTimestamp == nil {
			desiredMachines += machineDeployment.Spec.Replicas
		}
	}

	if int32(registeredMachine) < desiredMachines {
		return helper.ModifyCondition(condition, corev1.ConditionFalse, "MissingNodes", fmt.Sprintf("Too less worker nodes registered to the cluster (%d/%d).", registeredMachine, desiredMachines))
	}

	// Check whether every machine object reports that the machine is in Running state.
	machineList, err := b.K8sSeedClient.MachineClientset().MachineV1alpha1().Machines(b.Shoot.SeedNamespace).List(metav1.ListOptions{})
	if err != nil {
		return helper.ModifyCondition(condition, corev1.ConditionUnknown, "FetchMachineListFailed", err.Error())
	}
	for _, machine := range machineList.Items {
		var (
			phase         = machine.Status.CurrentStatus.Phase
			lastOperation = machine.Status.LastOperation
		)
		if phase != machinev1alpha1.MachineRunning {
			return helper.ModifyCondition(condition, corev1.ConditionFalse, "MachineNotRunning", fmt.Sprintf("Machine %s is in phase %s (description: %s)", machine.Name, phase, lastOperation.Description))
		}
	}

	return helper.ModifyCondition(condition, corev1.ConditionTrue, "EveryNodeReady", fmt.Sprintf("Every node registered to the cluster is ready (%d/%d).", registeredMachine, desiredMachines))
}

// CheckConditionSystemComponentsHealthy checks whether every container in the kube-system namespace of the Shoot cluster is in "Running"
// state and that the number of available replicas per deployment matches the number of actual replicas (i.e., the number of desired pods
// matches the number of actual running pods).
func (b *Botanist) CheckConditionSystemComponentsHealthy(condition *gardenv1beta1.Condition) *gardenv1beta1.Condition {
	// If the cluster has been hibernated then we do not want to check whether all system components are running (because there are not any
	// nodes/machines, i.e. this condition would be false everytime.
	if b.Shoot.Hibernated {
		return helper.ModifyCondition(condition, corev1.ConditionTrue, "ConditionNotChecked", "Shoot cluster has been hibernated.")
	}

	listOptions := metav1.ListOptions{
		LabelSelector: "origin=gardener",
	}

	// Check whether the number of availableReplicas matches the number of desired replicas for all our deployments.
	if updatedCondition, modified := verifyDeploymentHealthiness(condition, b.K8sShootClient, metav1.NamespaceSystem, listOptions); modified {
		return updatedCondition
	}

	// Check whether the number of running containers matching the number of actual containers within all our pods (i.e., everything we deploy
	// is running).
	if updatedCondition, modified := verifyPodHealthiness(condition, b.K8sShootClient, metav1.NamespaceSystem, listOptions); modified {
		return updatedCondition
	}

	return helper.ModifyCondition(condition, corev1.ConditionTrue, "AllPodsRunning", "All pods in the kube-system namespace of the Shoot cluster are running.")
}

// Helper functions

func verifyDeploymentHealthiness(condition *gardenv1beta1.Condition, k8sClient kubernetes.Client, namespace string, listOptions metav1.ListOptions) (*gardenv1beta1.Condition, bool) {
	deploymentList, err := k8sClient.ListDeployments(namespace, listOptions)
	if err != nil {
		return helper.ModifyCondition(condition, corev1.ConditionUnknown, "FetchDeploymentListFailed", err.Error()), true
	}

	for _, deployment := range deploymentList {
		if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas != deployment.Status.AvailableReplicas {
			return helper.ModifyCondition(condition, corev1.ConditionFalse, "DeploymentUnavailable", fmt.Sprintf("Deployment %s has not yet the desired number of available pods.", deployment.Name)), true
		}
	}

	return condition, false
}

func verifyPodHealthiness(condition *gardenv1beta1.Condition, k8sClient kubernetes.Client, namespace string, listOptions metav1.ListOptions) (*gardenv1beta1.Condition, bool) {
	podList, err := k8sClient.ListPods(namespace, listOptions)
	if err != nil {
		return helper.ModifyCondition(condition, corev1.ConditionUnknown, "FetchPodListFailed", err.Error()), true
	}

	for _, pod := range podList.Items {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Waiting != nil {
				return helper.ModifyCondition(condition, corev1.ConditionFalse, "ContainerWaiting", fmt.Sprintf("Container %s of pod %s is waiting to start.", containerStatus.Name, pod.Name)), true
			}
			if containerStatus.State.Running != nil && !containerStatus.Ready {
				return helper.ModifyCondition(condition, corev1.ConditionFalse, "ContainerNotReady", fmt.Sprintf("Container %s of pod %s is not ready yet.", containerStatus.Name, pod.Name)), true
			}
			if containerStatus.State.Terminated != nil && containerStatus.State.Terminated.Reason != "Completed" {
				return helper.ModifyCondition(condition, corev1.ConditionFalse, "ContainerNotRunning", fmt.Sprintf("Container %s of pod %s is not in running state.", containerStatus.Name, pod.Name)), true
			}
		}
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodSucceeded {
			return helper.ModifyCondition(condition, corev1.ConditionFalse, "PodNotRunning", fmt.Sprintf("Pod %s is in phase %s", pod.Name, pod.Status.Phase)), true
		}
	}

	return condition, false
}
