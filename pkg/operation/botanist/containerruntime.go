// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"github.com/gardener/gardener/pkg/operation/botanist/extensions/containerruntime"
	"github.com/gardener/gardener/pkg/operation/shoot"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultContainerRuntime creates the default deployer for the ContainerRuntime custom resource.
func (b *Botanist) DefaultContainerRuntime(seedClient client.Client) shoot.ContainerRuntime {
	return containerruntime.New(
		b.Logger,
		seedClient,
		&containerruntime.Values{
			Namespace: b.Shoot.SeedNamespace,
			Workers:   b.Shoot.Info.Spec.Provider.Workers,
		},
		containerruntime.DefaultInterval,
		containerruntime.DefaultSevereThreshold,
		containerruntime.DefaultTimeout,
	)
}

// DeployContainerRuntime deploys the ContainerRuntime custom resources and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration
func (b *Botanist) DeployContainerRuntime(ctx context.Context) error {
	if b.isRestorePhase() {
		return b.Shoot.Components.Extensions.ContainerRuntime.Restore(ctx, b.ShootState)
	}
	return b.Shoot.Components.Extensions.ContainerRuntime.Deploy(ctx)
}
