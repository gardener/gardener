// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kubecontrollermanager "github.com/gardener/gardener/pkg/component/kubernetes/controllermanager"
	"github.com/gardener/gardener/pkg/component/shared"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DefaultKubeControllerManager returns a deployer for the kube-controller-manager.
func (b *Botanist) DefaultKubeControllerManager() (kubecontrollermanager.Interface, error) {
	return shared.NewKubeControllerManager(
		b.Logger,
		b.SeedClientSet,
		b.Shoot.ControlPlaneNamespace,
		b.Seed.KubernetesVersion,
		b.Shoot.KubernetesVersion,
		b.SecretsManager,
		"",
		b.Shoot.GetInfo().Spec.Kubernetes.KubeControllerManager,
		v1beta1constants.PriorityClassNameShootControlPlane300,
		b.Shoot.IsWorkerless,
		metav1.HasAnnotation(b.Shoot.GetInfo().ObjectMeta, v1beta1constants.ShootAlphaControlPlaneScaleDownDisabled),
		nil,
		kubecontrollermanager.ControllerWorkers{},
		kubecontrollermanager.ControllerSyncPeriods{},
		nil,
	)
}

// DeployKubeControllerManager deploys the Kubernetes Controller Manager.
func (b *Botanist) DeployKubeControllerManager(ctx context.Context) error {
	replicaCount, err := b.determineControllerReplicas(ctx, v1beta1constants.DeploymentNameKubeControllerManager, 1, true)
	if err != nil {
		return err
	}
	if b.Shoot.RunsControlPlane() {
		replicaCount = 0
	}

	b.Shoot.Components.ControlPlane.KubeControllerManager.SetReplicaCount(replicaCount)
	b.Shoot.Components.ControlPlane.KubeControllerManager.SetRuntimeConfig(b.Shoot.Components.ControlPlane.KubeAPIServer.GetValues().RuntimeConfig)
	b.Shoot.Components.ControlPlane.KubeControllerManager.SetServiceNetworks(b.Shoot.Networks.Services)
	b.Shoot.Components.ControlPlane.KubeControllerManager.SetPodNetworks(b.Shoot.Networks.Pods)

	return b.Shoot.Components.ControlPlane.KubeControllerManager.Deploy(ctx)
}

// WaitForKubeControllerManagerToBeActive waits for the kube controller manager of a Shoot cluster has acquired leader election, thus is active.
func (b *Botanist) WaitForKubeControllerManagerToBeActive(ctx context.Context) error {
	b.Shoot.Components.ControlPlane.KubeControllerManager.SetShootClient(b.ShootClientSet.Client())

	return b.Shoot.Components.ControlPlane.KubeControllerManager.WaitForControllerToBeActive(ctx)
}

// ScaleKubeControllerManagerToOne scales kube-controller-manager replicas to one.
func (b *Botanist) ScaleKubeControllerManagerToOne(ctx context.Context) error {
	return kubernetesutils.ScaleDeployment(ctx, b.SeedClientSet.Client(), client.ObjectKey{Namespace: b.Shoot.ControlPlaneNamespace, Name: v1beta1constants.DeploymentNameKubeControllerManager}, 1)
}
