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
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WaitUntilKubeAPIServerServiceIsReady waits until the external load balancer of the kube-apiserver has
// been created (i.e., its ingress information has been updated in the service status).
func (b *Botanist) WaitUntilKubeAPIServerServiceIsReady(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	return retry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		loadBalancerIngress, err := kutil.GetLoadBalancerIngress(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace, gardencorev1alpha1.DeploymentNameKubeAPIServer)
		if err != nil {
			b.Logger.Info("Waiting until the kube-apiserver service is ready...")
			// TODO(AC): This is a quite optimistic check / we should differentiate here
			return retry.MinorError(fmt.Errorf("kube-apiserver service is not ready: %v", err))
		}
		b.Operation.APIServerAddress = loadBalancerIngress
		return retry.Ok()
	})
}

// WaitUntilEtcdReady waits until the etcd statefulsets indicate readiness in their statuses.
func (b *Botanist) WaitUntilEtcdReady(ctx context.Context) error {
	return retry.UntilTimeout(ctx, 5*time.Second, 300*time.Second, func(ctx context.Context) (done bool, err error) {
		statefulSetList := &appsv1.StatefulSetList{}
		err = b.K8sSeedClient.Client().List(ctx, statefulSetList,
			client.InNamespace(b.Shoot.SeedNamespace),
			client.MatchingLabels(map[string]string{"app": "etcd-statefulset"}))
		if err != nil {
			return retry.SevereError(err)
		}
		if n := len(statefulSetList.Items); n < 2 {
			b.Logger.Info("Waiting until the etcd statefulsets gets created...")
			return retry.MinorError(fmt.Errorf("only %d/%d etcd stateful sets found", n, 2))
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
			return retry.Ok()
		}

		b.Logger.Info("Waiting until the both etcd statefulsets are ready...")
		return retry.MinorError(fmt.Errorf("not all etcd stateful sets are ready"))
	})
}

// WaitUntilKubeAPIServerReady waits until the kube-apiserver pod(s) indicate readiness in their statuses.
func (b *Botanist) WaitUntilKubeAPIServerReady(ctx context.Context) error {
	return retry.UntilTimeout(ctx, 5*time.Second, 300*time.Second, func(ctx context.Context) (done bool, err error) {

		deploy := &appsv1.Deployment{}
		if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, gardencorev1alpha1.DeploymentNameKubeAPIServer), deploy); err != nil {
			return retry.SevereError(err)
		}
		if deploy.Generation != deploy.Status.ObservedGeneration {
			return retry.MinorError(fmt.Errorf("kube-apiserver not observed at latest generation (%d/%d)",
				deploy.Status.ObservedGeneration, deploy.Generation))
		}

		replicas := int32(0)
		if deploy.Spec.Replicas != nil {
			replicas = *deploy.Spec.Replicas
		}
		if replicas != deploy.Status.UpdatedReplicas {
			return retry.MinorError(fmt.Errorf("kube-apiserver does not have enough updated replicas (%d/%d)",
				deploy.Status.UpdatedReplicas, replicas))
		}
		if replicas != deploy.Status.Replicas {
			return retry.MinorError(fmt.Errorf("kube-apiserver deployment has outdated replicas"))
		}
		if replicas != deploy.Status.AvailableReplicas {
			return retry.MinorError(fmt.Errorf("kube-apiserver does not have enough available replicas (%d/%d",
				deploy.Status.AvailableReplicas, replicas))
		}

		return retry.Ok()
	})
}

// WaitUntilVPNConnectionExists waits until a port forward connection to the vpn-shoot pod in the kube-system
// namespace of the Shoot cluster can be established.
func (b *Botanist) WaitUntilVPNConnectionExists(ctx context.Context) error {
	return retry.UntilTimeout(ctx, 5*time.Second, 900*time.Second, func(ctx context.Context) (done bool, err error) {
		podList := &corev1.PodList{}
		err = b.K8sShootClient.Client().List(ctx, podList,
			client.InNamespace(metav1.NamespaceSystem),
			client.MatchingLabels(map[string]string{"app": "vpn-shoot"}))
		if err != nil {
			return retry.SevereError(err)
		}
		var vpnPod *corev1.Pod
		for _, pod := range podList.Items {
			if pod.Status.Phase == corev1.PodRunning {
				vpnPod = &pod
				break
			}
		}
		if vpnPod == nil {
			b.Logger.Info("Waiting until a running vpn-shoot pod exists in the Shoot cluster...")
			return retry.MinorError(fmt.Errorf("no vpn-shoot pod running in the Shoot cluster"))
		}

		if err := b.K8sShootClient.CheckForwardPodPort(vpnPod.ObjectMeta.Namespace, vpnPod.ObjectMeta.Name, 0, 22); err != nil {
			b.Logger.Info("Waiting until the VPN connection has been established...")
			return retry.MinorError(fmt.Errorf("could not forward to vpn-shoot pod: %v", err))
		}

		b.Logger.Info("VPN connection has been established.")
		return retry.Ok()
	})
}

// WaitUntilSeedNamespaceDeleted waits until the namespace of the Shoot cluster within the Seed cluster is deleted.
func (b *Botanist) WaitUntilSeedNamespaceDeleted(ctx context.Context) error {
	return b.waitUntilNamespaceDeleted(ctx, b.Shoot.SeedNamespace)
}

// WaitUntilBackupNamespaceDeleted waits until the namespace for the backup of Shoot cluster within the Seed cluster is deleted.
func (b *Botanist) WaitUntilBackupNamespaceDeleted(ctx context.Context) error {
	return b.waitUntilNamespaceDeleted(ctx, common.GenerateBackupNamespaceName(b.BackupInfrastructure.Name))
}

// WaitUntilNamespaceDeleted waits until the <namespace> within the Seed cluster is deleted.
func (b *Botanist) waitUntilNamespaceDeleted(ctx context.Context, namespace string) error {
	return retry.UntilTimeout(ctx, 5*time.Second, 900*time.Second, func(ctx context.Context) (done bool, err error) {
		if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKey{Name: namespace}, &corev1.Namespace{}); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.SevereError(err)
		}
		b.Logger.Infof("Waiting until the namespace '%s' has been cleaned up and deleted in the Seed cluster...", namespace)
		return retry.MinorError(fmt.Errorf("namespace %q is not yet cleaned up", namespace))
	})
}

// WaitUntilClusterAutoscalerDeleted waits until the cluster-autoscaler deployment within the Seed cluster has
// been deleted.
func (b *Botanist) WaitUntilClusterAutoscalerDeleted(ctx context.Context) error {
	return retry.UntilTimeout(ctx, 5*time.Second, 600*time.Second, func(ctx context.Context) (done bool, err error) {
		if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, gardencorev1alpha1.DeploymentNameClusterAutoscaler), &appsv1.Deployment{}); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.SevereError(err)
		}
		b.Logger.Infof("Waiting until the %s has been deleted in the Seed cluster...", gardencorev1alpha1.DeploymentNameClusterAutoscaler)
		return retry.MinorError(fmt.Errorf("deployment %q is still present", gardencorev1alpha1.DeploymentNameClusterAutoscaler))
	})
}

// WaitForControllersToBeActive checks whether kube-controller-manager has
// recently written to the Endpoint object holding the leader information. If yes, it is active.
func (b *Botanist) WaitForControllersToBeActive(ctx context.Context) error {
	type controllerInfo struct {
		name   string
		labels map[string]string
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

	// Check whether the kube-controller-manager deployment exists
	if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, gardencorev1alpha1.DeploymentNameKubeControllerManager), &appsv1.Deployment{}); err == nil {
		controllers = append(controllers, controllerInfo{
			name: gardencorev1alpha1.DeploymentNameKubeControllerManager,
			labels: map[string]string{
				"app":  "kubernetes",
				"role": "controller-manager",
			},
		})
	} else if client.IgnoreNotFound(err) != nil {
		return err
	}

	return retry.UntilTimeout(context.TODO(), pollInterval, 90*time.Second, func(ctx context.Context) (done bool, err error) {
		var (
			wg  sync.WaitGroup
			out = make(chan *checkOutput)
		)

		for _, controller := range controllers {
			wg.Add(1)

			go func(controller controllerInfo) {
				defer wg.Done()

				podList := &corev1.PodList{}
				err := b.K8sSeedClient.Client().List(ctx, podList,
					client.InNamespace(b.Shoot.SeedNamespace),
					client.MatchingLabels(controller.labels))
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
				return retry.SevereError(fmt.Errorf("could not check whether controller %s is active: %+v", result.controllerName, result.err))
			}
			if !result.ready {
				return retry.MinorError(fmt.Errorf("controller %s is not active", result.controllerName))
			}
		}

		return retry.Ok()
	})
}

// WaitUntilNodesDeleted waits until no nodes exist in the shoot cluster anymore.
func (b *Botanist) WaitUntilNodesDeleted(ctx context.Context) error {
	return retry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		nodesList := &corev1.NodeList{}
		if err := b.K8sShootClient.Client().List(ctx, nodesList); err != nil {
			return retry.SevereError(err)
		}

		if len(nodesList.Items) == 0 {
			return retry.Ok()
		}

		b.Logger.Infof("Waiting until all nodes have been deleted in the shoot cluster...")
		return retry.MinorError(fmt.Errorf("not all nodes have been deleted in the shoot cluster"))
	})
}

// WaitUntilBackupEntryInGardenReconciled waits until the backup entry within the garden cluster has
// been reconciled.
func (b *Botanist) WaitUntilBackupEntryInGardenReconciled(ctx context.Context) error {
	return retry.UntilTimeout(ctx, 5*time.Second, 600*time.Second, func(ctx context.Context) (done bool, err error) {
		be := &gardencorev1alpha1.BackupEntry{}
		if err := b.K8sGardenClient.Client().Get(ctx, kutil.Key(b.Shoot.Info.Namespace, common.GenerateBackupEntryName(b.Shoot.SeedNamespace, b.Shoot.Info.Status.UID)), be); err != nil {
			return retry.SevereError(err)
		}
		if be.Status.LastOperation != nil {
			if be.Status.LastOperation.State == gardencorev1alpha1.LastOperationStateSucceeded {
				b.Logger.Info("Backup entry has been successfully reconciled.")
				return retry.Ok()
			}
			if be.Status.LastOperation.State == gardencorev1alpha1.LastOperationStateError {
				b.Logger.Info("Backup entry has been reconciled with error.")
				return retry.SevereError(errors.New(be.Status.LastError.Description))
			}
		}
		b.Logger.Info("Waiting until the backup entry has been reconciled in the Garden cluster...")
		return retry.MinorError(fmt.Errorf("backup entry %q has not yet been reconciled", be.Name))
	})
}
