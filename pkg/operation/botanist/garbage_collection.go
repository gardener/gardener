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

	"github.com/gardener/gardener/pkg/client/kubernetes"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// PerformGarbageCollectionSeed performs garbage collection in the Shoot namespace in the Seed cluster,
// i.e., it deletes old machine sets which have a desired=actual=0 replica count.
func (b *Botanist) PerformGarbageCollectionSeed() error {
	if err := b.deleteEvictedPods(b.K8sSeedClient, b.Shoot.SeedNamespace); err != nil {
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
	return b.deleteEvictedPods(b.K8sShootClient, metav1.NamespaceSystem)
}

// deleteEvictedPods determine pods in state 'Evicted' in a given namespace and delete them.
func (b *Botanist) deleteEvictedPods(client kubernetes.Client, namespace string) error {
	podList, err := client.ListPods(namespace, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, pod := range podList.Items {
		var (
			name   = pod.ObjectMeta.Name
			reason = pod.Status.Reason
		)
		if reason != "" && strings.Contains(reason, "Evicted") {
			b.Logger.Debugf("Deleting pod %s as its reason is %s.", name, reason)
			err := client.DeletePod(namespace, name)
			if apierrors.IsNotFound(err) {
				continue
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}
