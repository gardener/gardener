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

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/infrastructure"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/secrets"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultInfrastructure creates the default deployer for the Infrastructure custom resource.
func (b *Botanist) DefaultInfrastructure(seedClient client.Client) infrastructure.Interface {
	return infrastructure.New(
		b.Logger,
		seedClient,
		&infrastructure.Values{
			Namespace:         b.Shoot.SeedNamespace,
			Name:              b.Shoot.Info.Name,
			Type:              b.Shoot.Info.Spec.Provider.Type,
			ProviderConfig:    b.Shoot.Info.Spec.Provider.InfrastructureConfig,
			Region:            b.Shoot.Info.Spec.Region,
			AnnotateOperation: controllerutils.HasTask(b.Shoot.Info.Annotations, common.ShootTaskDeployInfrastructure) || b.isRestorePhase(),
		},
		infrastructure.DefaultInterval,
		infrastructure.DefaultSevereThreshold,
		infrastructure.DefaultTimeout,
	)
}

// DeployInfrastructure deploys the Infrastructure custom resource and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration.
func (b *Botanist) DeployInfrastructure(ctx context.Context) error {
	b.Shoot.Components.Extensions.Infrastructure.SetSSHPublicKey(b.Secrets[v1beta1constants.SecretNameSSHKeyPair].Data[secrets.DataKeySSHAuthorizedKeys])

	if b.isRestorePhase() {
		return b.Shoot.Components.Extensions.Infrastructure.Restore(ctx, b.ShootState)
	}

	return b.Shoot.Components.Extensions.Infrastructure.Deploy(ctx)
}

// WaitForInfrastructure waits until the infrastructure reconciliation has finished and extracts the provider status
// out of it.
func (b *Botanist) WaitForInfrastructure(ctx context.Context) error {
	if err := b.Shoot.Components.Extensions.Infrastructure.Wait(ctx); err != nil {
		return err
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
