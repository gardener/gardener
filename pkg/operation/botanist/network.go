// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/network"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultNetwork creates the default deployer for the Network custom resource.
func (b *Botanist) DefaultNetwork(seedClient client.Client) component.DeployMigrateWaiter {
	return network.New(
		b.Logger,
		seedClient,
		&network.Values{
			Namespace:      b.Shoot.SeedNamespace,
			Name:           b.Shoot.Info.Name,
			Type:           b.Shoot.Info.Spec.Networking.Type,
			ProviderConfig: b.Shoot.Info.Spec.Networking.ProviderConfig,
			PodCIDR:        b.Shoot.Networks.Pods,
			ServiceCIDR:    b.Shoot.Networks.Services,
		},
		network.DefaultInterval,
		network.DefaultSevereThreshold,
		network.DefaultTimeout,
	)
}

// DeployNetwork deploys the Network custom resource and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration
func (b *Botanist) DeployNetwork(ctx context.Context) error {
	if b.isRestorePhase() {
		return b.Shoot.Components.Extensions.Network.Restore(ctx, b.ShootState)
	}

	return b.Shoot.Components.Extensions.Network.Deploy(ctx)
}
