// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"net"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/operation/botanist/component/shared"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DefaultKubeControllerManager returns a deployer for the kube-controller-manager.
func (b *Botanist) DefaultKubeControllerManager() (kubecontrollermanager.Interface, error) {
	hvpaEnabled := features.DefaultFeatureGate.Enabled(features.HVPA)
	if b.ManagedSeed != nil {
		hvpaEnabled = features.DefaultFeatureGate.Enabled(features.HVPAForShootedSeed)
	}

	scaleDownUpdateMode := hvpav1alpha1.UpdateModeAuto
	if metav1.HasAnnotation(b.Shoot.GetInfo().ObjectMeta, v1beta1constants.ShootAlphaControlPlaneScaleDownDisabled) {
		scaleDownUpdateMode = hvpav1alpha1.UpdateModeOff
	}

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
		b.ImageVector,
		b.SecretsManager,
		"",
		b.Shoot.GetInfo().Spec.Kubernetes.KubeControllerManager,
		v1beta1constants.PriorityClassNameShootControlPlane300,
		b.Shoot.IsWorkerless,
		&kubecontrollermanager.HVPAConfig{
			Enabled:             hvpaEnabled,
			ScaleDownUpdateMode: &scaleDownUpdateMode,
		},
		pods,
		services,
		nil,
		kubecontrollermanager.ControllerWorkers{},
		kubecontrollermanager.ControllerSyncPeriods{},
	)
}

// DeployKubeControllerManager deploys the Kubernetes Controller Manager.
func (b *Botanist) DeployKubeControllerManager(ctx context.Context) error {
	replicaCount, err := b.determineControllerReplicas(ctx, v1beta1constants.DeploymentNameKubeControllerManager, 1, true)
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.KubeControllerManager.SetReplicaCount(replicaCount)

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
