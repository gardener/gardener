// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"net"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubecontrollermanager "github.com/gardener/gardener/pkg/component/kubernetes/controllermanager"
	"github.com/gardener/gardener/pkg/component/shared"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DefaultKubeControllerManager returns a deployer for the kube-controller-manager.
func (b *Botanist) DefaultKubeControllerManager() (kubecontrollermanager.Interface, error) {
	var services, pods *net.IPNet
	if b.Shoot.Networks != nil {
		services = b.Shoot.Networks.Services
		pods = b.Shoot.Networks.Pods
	}

	return shared.NewKubeControllerManager(
		b.Logger,
		b.SeedClientSet,
		b.Shoot.SeedNamespace,
		b.Seed.KubernetesVersion,
		b.Shoot.KubernetesVersion,
		b.SecretsManager,
		"",
		b.Shoot.GetInfo().Spec.Kubernetes.KubeControllerManager,
		v1beta1constants.PriorityClassNameShootControlPlane300,
		b.Shoot.IsWorkerless,
		metav1.HasAnnotation(b.Shoot.GetInfo().ObjectMeta, v1beta1constants.ShootAlphaControlPlaneScaleDownDisabled),
		pods,
		services,
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
	b.Shoot.Components.ControlPlane.KubeControllerManager.SetReplicaCount(replicaCount)
	b.Shoot.Components.ControlPlane.KubeControllerManager.SetRuntimeConfig(b.Shoot.Components.ControlPlane.KubeAPIServer.GetValues().RuntimeConfig)

	return b.Shoot.Components.ControlPlane.KubeControllerManager.Deploy(ctx)
}

// WaitForKubeControllerManagerToBeActive waits for the kube controller manager of a Shoot cluster has acquired leader election, thus is active.
func (b *Botanist) WaitForKubeControllerManagerToBeActive(ctx context.Context) error {
	b.Shoot.Components.ControlPlane.KubeControllerManager.SetShootClient(b.ShootClientSet.Client())

	return b.Shoot.Components.ControlPlane.KubeControllerManager.WaitForControllerToBeActive(ctx)
}

// ScaleKubeControllerManagerToOne scales kube-controller-manager replicas to one.
func (b *Botanist) ScaleKubeControllerManagerToOne(ctx context.Context) error {
	return kubernetes.ScaleDeployment(ctx, b.SeedClientSet.Client(), kubernetesutils.Key(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeControllerManager), 1)
}
