// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/component/extensions/infrastructure"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/secrets"
)

// DefaultInfrastructure creates the default deployer for the Infrastructure custom resource.
func (b *Botanist) DefaultInfrastructure() infrastructure.Interface {
	return infrastructure.New(
		b.Logger,
		b.SeedClientSet.Client(),
		&infrastructure.Values{
			Namespace:         b.Shoot.ControlPlaneNamespace,
			Name:              b.Shoot.GetInfo().Name,
			Type:              b.Shoot.GetInfo().Spec.Provider.Type,
			ProviderConfig:    b.Shoot.GetInfo().Spec.Provider.InfrastructureConfig,
			Region:            b.Shoot.GetInfo().Spec.Region,
			AnnotateOperation: controllerutils.HasTask(b.Shoot.GetInfo().Annotations, v1beta1constants.ShootTaskDeployInfrastructure) || b.IsRestorePhase(),
		},
		infrastructure.DefaultInterval,
		infrastructure.DefaultSevereThreshold,
		infrastructure.DefaultTimeout,
	)
}

// DeployInfrastructure deploys the Infrastructure custom resource and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration.
func (b *Botanist) DeployInfrastructure(ctx context.Context) error {
	if v1beta1helper.ShootEnablesSSHAccess(b.Shoot.GetInfo()) {
		sshKeypairSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameSSHKeyPair)
		if !found {
			return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameSSHKeyPair)
		}
		b.Shoot.Components.Extensions.Infrastructure.SetSSHPublicKey(sshKeypairSecret.Data[secrets.DataKeySSHAuthorizedKeys])
	}

	if b.IsRestorePhase() {
		return b.Shoot.Components.Extensions.Infrastructure.Restore(ctx, b.Shoot.GetShootState())
	}

	return b.Shoot.Components.Extensions.Infrastructure.Deploy(ctx)
}

// WaitForInfrastructure waits until the infrastructure reconciliation has finished and extracts the provider status
// out of it.
func (b *Botanist) WaitForInfrastructure(ctx context.Context) error {
	if err := b.Shoot.Components.Extensions.Infrastructure.Wait(ctx); err != nil {
		return err
	}

	networkingStatus := &gardencorev1beta1.NetworkingStatus{}
	if nodesCIDRs := b.Shoot.Components.Extensions.Infrastructure.NodesCIDRs(); len(nodesCIDRs) > 0 {
		// Only update node CIDR if it's not already set.
		if b.Shoot.GetInfo().Spec.Networking.Nodes == nil {
			if err := b.Shoot.UpdateInfo(ctx, b.GardenClient, true, false, func(shoot *gardencorev1beta1.Shoot) error {
				shoot.Spec.Networking.Nodes = &nodesCIDRs[0]
				return nil
			}); err != nil {
				return err
			}
		}

		networkingStatus.Nodes = nodesCIDRs
	}

	if servicesCIDRs := b.Shoot.Components.Extensions.Infrastructure.ServicesCIDRs(); len(servicesCIDRs) > 0 {
		networkingStatus.Services = servicesCIDRs
	}
	if podsCIDRs := b.Shoot.Components.Extensions.Infrastructure.PodsCIDRs(); len(podsCIDRs) > 0 {
		networkingStatus.Pods = podsCIDRs
	}
	if egressCIDRs := b.Shoot.Components.Extensions.Infrastructure.EgressCIDRs(); len(egressCIDRs) > 0 {
		networkingStatus.EgressCIDRs = egressCIDRs
	}

	if err := b.Shoot.UpdateInfoStatus(ctx, b.GardenClient, true, false, func(shoot *gardencorev1beta1.Shoot) error {
		shoot.Status.Networking = networkingStatus
		return nil
	}); err != nil {
		return err
	}

	if err := extensions.SyncClusterResourceToSeed(ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, b.Shoot.GetInfo(), nil, nil); err != nil {
		return err
	}

	networks, err := shoot.ToNetworks(b.Shoot.GetInfo(), b.Shoot.IsWorkerless)
	if err != nil {
		return err
	}
	b.Shoot.Networks = networks
	return nil
}
