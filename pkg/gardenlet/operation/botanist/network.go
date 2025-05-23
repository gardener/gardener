// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/network"
)

// DefaultNetwork creates the default deployer for the Network custom resource.
func (b *Botanist) DefaultNetwork() network.Interface {
	var ipFamilies []extensionsv1alpha1.IPFamily
	for _, ipFamily := range b.Shoot.GetInfo().Spec.Networking.IPFamilies {
		ipFamilies = append(ipFamilies, extensionsv1alpha1.IPFamily(ipFamily))
	}

	return network.New(
		b.Logger,
		b.SeedClientSet.Client(),
		&network.Values{
			Namespace:      b.Shoot.ControlPlaneNamespace,
			Name:           b.Shoot.GetInfo().Name,
			Type:           *b.Shoot.GetInfo().Spec.Networking.Type,
			IPFamilies:     ipFamilies,
			ProviderConfig: b.Shoot.GetInfo().Spec.Networking.ProviderConfig,
		},
		network.DefaultInterval,
		network.DefaultSevereThreshold,
		network.DefaultTimeout,
	)
}

// DeployNetwork deploys the Network custom resource and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration
func (b *Botanist) DeployNetwork(ctx context.Context) error {
	b.Shoot.Components.Extensions.Network.SetPodCIDRs(b.Shoot.Networks.Pods)
	b.Shoot.Components.Extensions.Network.SetServiceCIDRs(b.Shoot.Networks.Services)

	if b.IsRestorePhase() {
		return b.Shoot.Components.Extensions.Network.Restore(ctx, b.Shoot.GetShootState())
	}

	return b.Shoot.Components.Extensions.Network.Deploy(ctx)
}
