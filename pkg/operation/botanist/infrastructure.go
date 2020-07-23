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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/infrastructure"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/secrets"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultInfrastructure creates the default deployer for the Infrastructure custom resource.
func (b *Botanist) DefaultInfrastructure(seedClient client.Client) shoot.Infrastructure {
	return infrastructure.New(
		b.Logger,
		seedClient,
		&infrastructure.Values{
			Namespace:                               b.Shoot.SeedNamespace,
			Name:                                    b.Shoot.Info.Name,
			Type:                                    b.Shoot.Info.Spec.Provider.Type,
			ProviderConfig:                          b.Shoot.Info.Spec.Provider.InfrastructureConfig,
			Region:                                  b.Shoot.Info.Spec.Region,
			IsInCreationPhase:                       b.Shoot.Info.Status.LastOperation != nil && b.Shoot.Info.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeCreate,
			IsWakingUp:                              !gardencorev1beta1helper.HibernationIsEnabled(b.Shoot.Info) && b.Shoot.Info.Status.IsHibernated,
			IsInRestorePhaseOfControlPlaneMigration: b.isRestorePhase(),
			DeploymentRequested:                     controllerutils.HasTask(b.Shoot.Info.Annotations, common.ShootTaskDeployInfrastructure),
		},
	)
}

// DeployInfrastructure deploys the Infrastructure custom resource and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration.
func (b *Botanist) DeployInfrastructure(ctx context.Context) error {
	b.Shoot.Components.Extensions.Infrastructure.SetSSHPublicKey(b.Secrets[v1beta1constants.SecretNameSSHKeyPair].Data[secrets.DataKeySSHAuthorizedKeys])

	if err := b.Shoot.Components.Extensions.Infrastructure.Deploy(ctx); err != nil {
		return err
	}

	if b.isRestorePhase() {
		return b.restoreExtensionObject(ctx, &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      b.Shoot.Info.Name,
				Namespace: b.Shoot.SeedNamespace,
			},
		}, extensionsv1alpha1.InfrastructureResource)
	}
	return nil
}

// WaitForInfrastructure waits until the infrastructure reconciliation has finished and extracts the provider status
// out of it.
func (b *Botanist) WaitForInfrastructure(ctx context.Context) error {
	if err := b.Shoot.Components.Extensions.Infrastructure.Wait(ctx); err != nil {
		return err
	}

	if providerStatus := b.Shoot.Components.Extensions.Infrastructure.ProviderStatus(); providerStatus != nil {
		b.Shoot.InfrastructureStatus = providerStatus.Raw
	}

	if nodesCIDR := b.Shoot.Components.Extensions.Infrastructure.NodesCIDR(); nodesCIDR != nil {
		shootCopy := b.Shoot.Info.DeepCopy()
		return b.UpdateShootAndCluster(ctx, shootCopy, func() error {
			shootCopy.Spec.Networking.Nodes = nodesCIDR
			return nil
		})
	}

	return nil
}
