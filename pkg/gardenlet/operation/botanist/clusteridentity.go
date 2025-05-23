// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/component/clusteridentity"
	"github.com/gardener/gardener/pkg/extensions"
)

// EnsureShootClusterIdentity ensures that Shoot's `status.clusterIdentity` field is set and updates the Cluster resource in
// the seed if necessary.
func (b *Botanist) EnsureShootClusterIdentity(ctx context.Context) error {
	if b.Shoot.GetInfo().Status.ClusterIdentity == nil {
		clusterIdentity := fmt.Sprintf("%s-%s-%s", b.Shoot.ControlPlaneNamespace, b.Shoot.GetInfo().Status.UID, b.GardenClusterIdentity)

		if err := b.Shoot.UpdateInfoStatus(ctx, b.GardenClient, false, false, func(shoot *gardencorev1beta1.Shoot) error {
			shoot.Status.ClusterIdentity = &clusterIdentity
			return nil
		}); err != nil {
			return err
		}

		if err := extensions.SyncClusterResourceToSeed(ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, b.Shoot.GetInfo(), nil, nil); err != nil {
			return err
		}
	}

	return nil
}

// DefaultClusterIdentity returns a deployer for the shoot's cluster-identity.
func (b *Botanist) DefaultClusterIdentity() clusteridentity.Interface {
	return clusteridentity.NewForShoot(b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, "")
}

// DeployClusterIdentity deploys the shoot's cluster-identity.
func (b *Botanist) DeployClusterIdentity(ctx context.Context) error {
	if v := b.Shoot.GetInfo().Status.ClusterIdentity; v != nil {
		b.Shoot.Components.SystemComponents.ClusterIdentity.SetIdentity(*v)
	}

	return b.Shoot.Components.SystemComponents.ClusterIdentity.Deploy(ctx)
}
