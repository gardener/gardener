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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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

	podList, err := b.K8sSeedClient.ListPods(b.Shoot.SeedNamespace, metav1.ListOptions{})
	if err != nil {
		return helper.ModifyCondition(condition, corev1.ConditionUnknown, "FetchPodListFailed", err.Error())
	}
	if len(podList.Items) < 6 {
		return helper.ModifyCondition(condition, corev1.ConditionFalse, "ControlPlaneIncomplete", "The control plane in the Shoot namespace is incomplete (Pod's are missing).")
	}
	for _, pod := range podList.Items {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Running == nil && containerStatus.State.Terminated != nil && containerStatus.State.Terminated.Reason != "Completed" {
				return helper.ModifyCondition(condition, corev1.ConditionFalse, "ContainerNotRunning", fmt.Sprintf("Container %s of pod %s is not in running state", containerStatus.Name, pod.Name))
			}
		}
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodSucceeded {
			return helper.ModifyCondition(condition, corev1.ConditionFalse, "PodNotRunning", fmt.Sprintf("Pod %s is in phase %s", pod.Name, pod.Status.Phase))
		}
	}

	return helper.ModifyCondition(condition, corev1.ConditionTrue, "AllContainersInRunningState", "All pods running the Shoot namespace in the Seed cluster are healthy.")
}

// CheckConditionEveryNodeReady checks whether every node registered at the Shoot cluster is in "Ready" state and
// that no node known to the IaaS is not registered to the Shoot's kube-apiserver.
func (b *Botanist) CheckConditionEveryNodeReady(condition *gardenv1beta1.Condition) *gardenv1beta1.Condition {
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

	var machineList unstructured.Unstructured
	if err := b.K8sSeedClient.MachineV1alpha1("GET", "machines", b.Shoot.SeedNamespace).Do().Into(&machineList); err != nil {
		return helper.ModifyCondition(condition, corev1.ConditionUnknown, "FetchMachineListFailed", err.Error())
	}
	if err := machineList.EachListItem(func(o runtime.Object) error {
		var (
			obj                                = o.(*unstructured.Unstructured)
			machineName                        = obj.GetName()
			machinePhase, machinePhaseFound, _ = unstructured.NestedString(obj.UnstructuredContent(), "status", "currentStatus", "phase")
			lastOpDescription, _, _            = unstructured.NestedString(obj.UnstructuredContent(), "status", "lastOperation", "description")
		)

		if !machinePhaseFound || machinePhase != "Running" {
			return fmt.Errorf("Machine %s is not running (phase: %s, description: %s)", machineName, machinePhase, lastOpDescription)
		}
		return nil
	}); err != nil {
		return helper.ModifyCondition(condition, corev1.ConditionFalse, "MachineUnhealthy", err.Error())
	}

	return helper.ModifyCondition(condition, corev1.ConditionTrue, "EveryNodeReady", "Every node registered to the cluster is ready.")
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

	// Check whether the number of availableReplicas matches the number of desired replicas.
	deploymentList, err := b.K8sShootClient.ListDeployments(metav1.NamespaceSystem, metav1.ListOptions{})
	if err != nil {
		return helper.ModifyCondition(condition, corev1.ConditionUnknown, "FetchPodListFailed", err.Error())
	}
	for _, deployment := range deploymentList {
		if *deployment.Spec.Replicas != deployment.Status.AvailableReplicas {
			return helper.ModifyCondition(condition, corev1.ConditionFalse, "NotAllPodsAvailable", fmt.Sprintf("Deployment %s has not yet the desired number of available pods.", deployment.Name))
		}
	}

	// Check whether the number of running containers matching the number of actual containers within the pods (i.e., everything is running).
	podList, err := b.K8sShootClient.ListPods(metav1.NamespaceSystem, metav1.ListOptions{
		LabelSelector: "origin=gardener",
	})
	if err != nil {
		return helper.ModifyCondition(condition, corev1.ConditionUnknown, "FetchPodListFailed", err.Error())
	}
	for _, pod := range podList.Items {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Running == nil && containerStatus.State.Terminated != nil && containerStatus.State.Terminated.Reason != "Completed" {
				return helper.ModifyCondition(condition, corev1.ConditionFalse, "ContainerNotRunning", fmt.Sprintf("Container %s of pod %s is not in running state.", containerStatus.Name, pod.Name))
			}
		}
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodSucceeded {
			return helper.ModifyCondition(condition, corev1.ConditionFalse, "PodNotRunning", fmt.Sprintf("Pod %s is in phase %s", pod.Name, pod.Status.Phase))
		}
	}

	return helper.ModifyCondition(condition, corev1.ConditionTrue, "AllContainersInKubeSystemInRunningState", "Every container in the kube-system namespace of the Shoot cluster is running.")
}
