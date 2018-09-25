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
	"strings"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// PerformGarbageCollectionSeed performs garbage collection in the Shoot namespace in the Seed cluster,
// i.e., it deletes old machine sets which have a desired=actual=0 replica count.
func (b *Botanist) PerformGarbageCollectionSeed() error {
	podList, err := b.K8sSeedClient.ListPods(b.Shoot.SeedNamespace, metav1.ListOptions{})
	if err != nil {
		return err
	}

	if err := b.deleteEvictedPods(b.K8sSeedClient, podList); err != nil {
		return err
	}
	if err := b.deleteStuckTerminatingPods(b.K8sSeedClient, podList); err != nil {
		return err
	}

	var machineSetList unstructured.Unstructured
	if err := b.K8sSeedClient.MachineV1alpha1("GET", "machinesets", b.Shoot.SeedNamespace).Do().Into(&machineSetList); err != nil {
		return err
	}
	return machineSetList.EachListItem(func(o runtime.Object) error {
		var (
			obj                                                          = o.(*unstructured.Unstructured)
			machineSetName                                               = obj.GetName()
			machineSetDesiredReplicas, machineSetDesiredReplicasFound, _ = unstructured.NestedInt64(obj.UnstructuredContent(), "spec", "replicas")
			machineSetActualReplicas, machineSetActualReplicasFound, _   = unstructured.NestedInt64(obj.UnstructuredContent(), "status", "replicas")
		)

		if !machineSetDesiredReplicasFound {
			machineSetDesiredReplicas = -1
		}
		if !machineSetActualReplicasFound {
			machineSetActualReplicas = -1
		}

		if machineSetDesiredReplicas == 0 && machineSetActualReplicas == 0 {
			b.Logger.Debugf("Deleting MachineSet %s as the number of desired and actual replicas is 0.", machineSetName)
			err := b.K8sSeedClient.MachineV1alpha1("DELETE", "machinesets", b.Shoot.SeedNamespace).Name(machineSetName).Do().Error()
			if apierrors.IsNotFound(err) {
				return nil
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// PerformGarbageCollectionShoot performs garbage collection in the kube-system namespace in the Shoot
// cluster, i.e., it deletes evicted pods (mitigation for https://github.com/kubernetes/kubernetes/issues/55051).
func (b *Botanist) PerformGarbageCollectionShoot() error {
	podList, err := b.K8sShootClient.ListPods(metav1.NamespaceSystem, metav1.ListOptions{})
	if err != nil {
		return err
	}

	if err := b.deleteEvictedPods(b.K8sShootClient, podList); err != nil {
		return err
	}
	return b.deleteStuckTerminatingPods(b.K8sShootClient, podList)
}

// deleteEvictedPods determines pods in state 'Evicted' in a given namespace and deletes them.
func (b *Botanist) deleteEvictedPods(client kubernetes.Client, podList *corev1.PodList) error {
	for _, pod := range podList.Items {
		if strings.Contains(pod.Status.Reason, "Evicted") {
			b.Logger.Debugf("Deleting pod %s as its reason is %s.", pod.Name, pod.Status.Reason)
			if err := client.DeletePod(pod.Namespace, pod.Name); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}

// deleteStuckTerminatingPods determines stuck pods in state 'Terminating' in a given namespace and deletes them.
func (b *Botanist) deleteStuckTerminatingPods(client kubernetes.Client, podList *corev1.PodList) error {
	var (
		now                 = metav1.Now()
		gardenerGracePeriod = time.Minute
	)

	for _, pod := range podList.Items {
		if pod.DeletionTimestamp != nil {
			podDeletionGracePeriod := gardenerGracePeriod
			if pod.DeletionGracePeriodSeconds != nil {
				podDeletionGracePeriod += time.Duration(*pod.DeletionGracePeriodSeconds) * time.Second
			}
			podMaximumAliveTime := pod.DeletionTimestamp.Add(podDeletionGracePeriod)

			if now.After(podMaximumAliveTime) {
				if err := client.DeletePodForcefully(pod.Namespace, pod.Name); err != nil {
					return nil
				}
			}
		}
	}
	return nil
}
