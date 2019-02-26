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
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// WaitUntilKubeAPIServerServiceIsReady waits until the external load balancer of the kube-apiserver has
// been created (i.e., its ingress information has been updated in the service status).
func (b *Botanist) WaitUntilKubeAPIServerServiceIsReady() error {
	var e error
	if err := wait.Poll(5*time.Second, 600*time.Second, func() (bool, error) {
		loadBalancerIngress, serviceStatusIngress, err := common.GetLoadBalancerIngress(b.K8sSeedClient, b.Shoot.SeedNamespace, common.KubeAPIServerDeploymentName)
		if err != nil {
			e = err
			b.Logger.Info("Waiting until the kube-apiserver service is ready...")
			return false, nil
		}
		b.Operation.APIServerAddress = loadBalancerIngress
		b.Operation.APIServerIngresses = serviceStatusIngress
		return true, nil
	}); err != nil {
		return e
	}
	return nil
}

// WaitUntilEtcdReady waits until the etcd statefulsets indicate readiness in their statuses.
func (b *Botanist) WaitUntilEtcdReady() error {
	return wait.Poll(5*time.Second, 300*time.Second, func() (bool, error) {
		statefulSetList, err := b.K8sSeedClient.ListStatefulSets(b.Shoot.SeedNamespace, metav1.ListOptions{
			LabelSelector: "app=etcd-statefulset",
		})
		if err != nil {
			return false, err
		}
		if len(statefulSetList.Items) < 2 {
			b.Logger.Info("Waiting until the etcd statefulsets gets created...")
			return false, nil
		}

		bothEtcdStatefulSetsReady := true
		for _, statefulSet := range statefulSetList.Items {
			if statefulSet.DeletionTimestamp != nil {
				continue
			}

			if statefulSet.Status.ReadyReplicas < 1 {
				bothEtcdStatefulSetsReady = false
				break
			}
		}

		if bothEtcdStatefulSetsReady {
			return true, nil
		}

		b.Logger.Info("Waiting until the both etcd statefulsets are ready...")
		return false, nil
	})
}

// WaitUntilKubeAPIServerReady waits until the kube-apiserver pod(s) indicate readiness in their statuses.
func (b *Botanist) WaitUntilKubeAPIServerReady() error {
	return wait.PollImmediate(5*time.Second, 300*time.Second, func() (bool, error) {
		podList, err := b.K8sSeedClient.ListPods(b.Shoot.SeedNamespace, metav1.ListOptions{
			LabelSelector: "app=kubernetes,role=apiserver",
		})
		if err != nil {
			return false, err
		}
		if len(podList.Items) == 0 {
			b.Logger.Info("Waiting until the kube-apiserver deployment gets created...")
			return false, nil
		}

		var ready bool
		for _, pod := range podList.Items {
			if pod.DeletionTimestamp != nil {
				continue
			}

			ready = false
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.Name == common.KubeAPIServerDeploymentName && containerStatus.Ready {
					ready = true
					break
				}
			}
		}

		if ready {
			return true, nil
		}

		b.Logger.Info("Waiting until the kube-apiserver deployment is ready...")
		return false, nil
	})
}

// WaitUntilBackupInfrastructureReconciled waits until the backup infrastructure within the garden cluster has
// been reconciled.
func (b *Botanist) WaitUntilBackupInfrastructureReconciled() error {
	return wait.PollImmediate(5*time.Second, 600*time.Second, func() (bool, error) {
		backupInfrastructures, err := b.K8sGardenClient.Garden().GardenV1beta1().BackupInfrastructures(b.Shoot.Info.Namespace).Get(common.GenerateBackupInfrastructureName(b.Shoot.SeedNamespace, b.Shoot.Info.Status.UID), metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if backupInfrastructures.Status.LastOperation != nil {
			if backupInfrastructures.Status.LastOperation.State == gardencorev1alpha1.LastOperationStateSucceeded {
				b.Logger.Info("Backup infrastructure has been successfully reconciled.")
				return true, nil
			}
			if backupInfrastructures.Status.LastOperation.State == gardencorev1alpha1.LastOperationStateError {
				b.Logger.Info("Backup infrastructure has been reconciled with error.")
				return true, errors.New(backupInfrastructures.Status.LastError.Description)
			}
		}
		b.Logger.Info("Waiting until the backup-infrastructure has been reconciled in the Garden cluster...")
		return false, nil
	})
}

// WaitUntilVPNConnectionExists waits until a port forward connection to the vpn-shoot pod in the kube-system
// namespace of the Shoot cluster can be established.
func (b *Botanist) WaitUntilVPNConnectionExists() error {
	return wait.PollImmediate(5*time.Second, 900*time.Second, func() (bool, error) {
		var vpnPod *corev1.Pod
		podList, err := b.K8sShootClient.ListPods(metav1.NamespaceSystem, metav1.ListOptions{
			LabelSelector: "app=vpn-shoot",
		})
		if err != nil {
			return false, err
		}
		for _, pod := range podList.Items {
			if pod.Status.Phase == corev1.PodRunning {
				vpnPod = &pod
				break
			}
		}
		if vpnPod == nil {
			b.Logger.Info("Waiting until a running vpn-shoot pod exists in the Shoot cluster...")
			return false, nil
		}
		if ok, err := b.K8sShootClient.CheckForwardPodPort(vpnPod.ObjectMeta.Namespace, vpnPod.ObjectMeta.Name, 0, 22); err == nil && ok {
			b.Logger.Info("VPN connection has been established.")
			return true, nil
		}
		b.Logger.Info("Waiting until the VPN connection has been established...")
		return false, nil
	})
}

// WaitUntilSeedNamespaceDeleted waits until the namespace of the Shoot cluster within the Seed cluster is deleted.
func (b *Botanist) WaitUntilSeedNamespaceDeleted() error {
	return b.waitUntilNamespaceDeleted(b.Shoot.SeedNamespace)
}

// WaitUntilBackupNamespaceDeleted waits until the namespace for the backup of Shoot cluster within the Seed cluster is deleted.
func (b *Botanist) WaitUntilBackupNamespaceDeleted() error {
	return b.waitUntilNamespaceDeleted(common.GenerateBackupNamespaceName(b.BackupInfrastructure.Name))
}

// WaitUntilNamespaceDeleted waits until the <namespace> within the Seed cluster is deleted.
func (b *Botanist) waitUntilNamespaceDeleted(namespace string) error {
	return wait.PollImmediate(5*time.Second, 900*time.Second, func() (bool, error) {
		if _, err := b.K8sSeedClient.GetNamespace(namespace); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		b.Logger.Infof("Waiting until the namespace '%s' has been cleaned up and deleted in the Seed cluster...", namespace)
		return false, nil
	})
}

// WaitUntilKubeAddonManagerDeleted waits until the kube-addon-manager deployment within the Seed cluster has
// been deleted.
func (b *Botanist) WaitUntilKubeAddonManagerDeleted() error {
	return wait.PollImmediate(5*time.Second, 600*time.Second, func() (bool, error) {
		if _, err := b.K8sSeedClient.GetDeployment(b.Shoot.SeedNamespace, common.KubeAddonManagerDeploymentName); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		b.Logger.Infof("Waiting until the %s has been deleted in the Seed cluster...", common.KubeAddonManagerDeploymentName)
		return false, nil
	})
}

// WaitUntilClusterAutoscalerDeleted waits until the cluster-autoscaler deployment within the Seed cluster has
// been deleted.
func (b *Botanist) WaitUntilClusterAutoscalerDeleted() error {
	return wait.PollImmediate(5*time.Second, 600*time.Second, func() (bool, error) {
		if _, err := b.K8sSeedClient.GetDeployment(b.Shoot.SeedNamespace, gardencorev1alpha1.DeploymentNameClusterAutoscaler); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		b.Logger.Infof("Waiting until the %s has been deleted in the Seed cluster...", gardencorev1alpha1.DeploymentNameClusterAutoscaler)
		return false, nil
	})
}

// WaitForControllersToBeActive checks whether the kube-controller-manager and the cloud-controller-manager have
// recently written to the Endpoint object holding the leader information. If yes, they are active.
func (b *Botanist) WaitForControllersToBeActive() error {
	type controllerInfo struct {
		name          string
		labelSelector string
	}

	type checkOutput struct {
		controllerName string
		ready          bool
		err            error
	}

	var (
		controllers  = []controllerInfo{}
		pollInterval = 5 * time.Second
	)

	// Check whether the cloud-controller-manager deployment exists
	if _, err := b.K8sSeedClient.GetDeployment(b.Shoot.SeedNamespace, common.CloudControllerManagerDeploymentName); err == nil {
		controllers = append(controllers, controllerInfo{
			name:          common.CloudControllerManagerDeploymentName,
			labelSelector: "app=kubernetes,role=cloud-controller-manager",
		})
	} else if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	// Check whether the kube-controller-manager deployment exists
	if _, err := b.K8sSeedClient.GetDeployment(b.Shoot.SeedNamespace, common.KubeControllerManagerDeploymentName); err == nil {
		controllers = append(controllers, controllerInfo{
			name:          common.KubeControllerManagerDeploymentName,
			labelSelector: "app=kubernetes,role=controller-manager",
		})
	} else if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return utils.Retry(pollInterval, 90*time.Second, func() (bool, bool, error) {
		var (
			wg  sync.WaitGroup
			out = make(chan *checkOutput)
		)

		for _, controller := range controllers {
			wg.Add(1)

			go func(controller controllerInfo) {
				defer wg.Done()

				podList, err := b.K8sSeedClient.ListPods(b.Shoot.SeedNamespace, metav1.ListOptions{
					LabelSelector: controller.labelSelector,
				})
				if err != nil {
					out <- &checkOutput{controllerName: controller.name, err: err}
					return
				}

				// Check that only one replica of the controller exists.
				if len(podList.Items) != 1 {
					b.Logger.Infof("Waiting for %s to have exactly one replica", controller.name)
					out <- &checkOutput{controllerName: controller.name}
					return
				}
				// Check that the existing replica is not in getting deleted.
				if podList.Items[0].DeletionTimestamp != nil {
					b.Logger.Infof("Waiting for a new replica of %s", controller.name)
					out <- &checkOutput{controllerName: controller.name}
					return
				}

				// Check if the controller is active by reading its leader election record.
				leaderElectionRecord, err := common.ReadLeaderElectionRecord(b.K8sShootClient, resourcelock.EndpointsResourceLock, metav1.NamespaceSystem, controller.name)
				if err != nil {
					out <- &checkOutput{controllerName: controller.name, err: err}
					return
				}

				if delta := metav1.Now().Sub(leaderElectionRecord.RenewTime.Time); delta <= pollInterval-time.Second {
					out <- &checkOutput{controllerName: controller.name, ready: true}
					return
				}

				b.Logger.Infof("Waiting for %s to be active", controller.name)
				out <- &checkOutput{controllerName: controller.name}
			}(controller)
		}

		go func() {
			wg.Wait()
			close(out)
		}()

		for result := range out {
			if result.err != nil {
				return false, true, fmt.Errorf("Could not check whether controller %s is active: %+v", result.controllerName, result.err)
			}
			if !result.ready {
				return false, false, fmt.Errorf("Controller %s is not active", result.controllerName)
			}
		}

		return true, false, nil
	})
}

// WaitUntilNodesDeleted waits until no nodes exist in the shoot cluster anymore.
func (b *Botanist) WaitUntilNodesDeleted(ctx context.Context) error {
	return utils.RetryUntil(ctx, 5*time.Second, func() (bool, bool, error) {
		nodesList, err := b.K8sShootClient.ListNodes(metav1.ListOptions{})
		if err != nil {
			return false, true, err
		}

		if len(nodesList.Items) == 0 {
			return true, false, nil
		}

		b.Logger.Infof("Waiting until all nodes have been deleted in the shoot cluster...")
		return false, false, nil
	})
}
