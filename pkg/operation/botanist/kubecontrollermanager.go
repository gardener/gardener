// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DefaultKubeControllerManager returns a deployer for the kube-controller-manager.
func (b *Botanist) DefaultKubeControllerManager() (kubecontrollermanager.Interface, error) {
	image, err := b.ImageVector.FindImage(charts.ImageNameKubeControllerManager, imagevector.RuntimeVersion(b.SeedVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	hvpaEnabled := gardenletfeatures.FeatureGate.Enabled(features.HVPA)
	if b.ManagedSeed != nil {
		hvpaEnabled = gardenletfeatures.FeatureGate.Enabled(features.HVPAForShootedSeed)
	}

	scaleDownUpdateMode := hvpav1alpha1.UpdateModeAuto
	if metav1.HasAnnotation(b.Shoot.GetInfo().ObjectMeta, v1beta1constants.ShootAlphaControlPlaneScaleDownDisabled) {
		scaleDownUpdateMode = hvpav1alpha1.UpdateModeOff
	}

	return kubecontrollermanager.New(
		b.Logger.WithField("component", "kube-controller-manager"),
		b.K8sSeedClient.Client(),
		b.Shoot.SeedNamespace,
		b.Shoot.KubernetesVersion,
		image.String(),
		b.Shoot.GetInfo().Spec.Kubernetes.KubeControllerManager,
		b.Shoot.Networks.Pods,
		b.Shoot.Networks.Services,
		&kubecontrollermanager.HVPAConfig{
			Enabled:             hvpaEnabled,
			ScaleDownUpdateMode: &scaleDownUpdateMode,
		},
	), nil
}

// DeployKubeControllerManager deploys the Kubernetes Controller Manager.
func (b *Botanist) DeployKubeControllerManager(ctx context.Context) error {
	replicaCount, err := b.determineKubeControllerManagerReplicas(ctx)
	if err != nil {
		return err
	}

	b.Shoot.Components.ControlPlane.KubeControllerManager.SetReplicaCount(replicaCount)
	b.Shoot.Components.ControlPlane.KubeControllerManager.SetSecrets(kubecontrollermanager.Secrets{
		CA:                component.Secret{Name: v1beta1constants.SecretNameCACluster, Checksum: b.LoadCheckSum(v1beta1constants.SecretNameCACluster)},
		ServiceAccountKey: component.Secret{Name: v1beta1constants.SecretNameServiceAccountKey, Checksum: b.LoadCheckSum(v1beta1constants.SecretNameServiceAccountKey)},
		Server:            component.Secret{Name: kubecontrollermanager.SecretNameServer, Checksum: b.LoadCheckSum(kubecontrollermanager.SecretNameServer)},
	})

	return b.Shoot.Components.ControlPlane.KubeControllerManager.Deploy(ctx)
}

// WaitForKubeControllerManagerToBeActive waits for the kube controller manager of a Shoot cluster has acquired leader election, thus is active.
func (b *Botanist) WaitForKubeControllerManagerToBeActive(ctx context.Context) error {
	b.Shoot.Components.ControlPlane.KubeControllerManager.SetShootClient(b.K8sShootClient.Client())

	return b.Shoot.Components.ControlPlane.KubeControllerManager.WaitForControllerToBeActive(ctx)
}

// ScaleKubeControllerManagerToOne scales kube-controller-manager replicas to one.
func (b *Botanist) ScaleKubeControllerManagerToOne(ctx context.Context) error {
	return kubernetes.ScaleDeployment(ctx, b.K8sSeedClient.Client(), kutil.Key(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeControllerManager), 1)
}

func (b *Botanist) determineKubeControllerManagerReplicas(ctx context.Context) (int32, error) {
	isCreateOrRestoreOperation := b.Shoot.GetInfo().Status.LastOperation != nil &&
		(b.Shoot.GetInfo().Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeCreate ||
			b.Shoot.GetInfo().Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeRestore)

	if isCreateOrRestoreOperation && b.Shoot.HibernationEnabled ||
		!isCreateOrRestoreOperation && b.Shoot.HibernationEnabled == b.Shoot.GetInfo().Status.IsHibernated {
		// shoot is being created or restored with .spec.hibernation.enabled=true or
		// shoot is being reconciled with .spec.hibernation.enabled=.status.isHibernated,
		// so keep the replicas which are already available.
		return kutil.CurrentReplicaCountForDeployment(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeControllerManager)
	}

	// shoot is being reconciled with .spec.hibernation.enabled!=.status.isHibernated, so deploy KCM. in case the
	// shoot is being hibernated then it will be scaled down to zero later after all machines are gone
	return 1, nil
}
