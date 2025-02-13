// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"github.com/gardener/gardener/pkg/component/extensions/containerruntime"
)

// DefaultContainerRuntime creates the default deployer for the ContainerRuntime custom resource.
func (b *Botanist) DefaultContainerRuntime() containerruntime.Interface {
	return containerruntime.New(
		b.Logger,
		b.SeedClientSet.Client(),
		&containerruntime.Values{
			Namespace: b.Shoot.ControlPlaneNamespace,
			Workers:   b.Shoot.GetInfo().Spec.Provider.Workers,
		},
		containerruntime.DefaultInterval,
		containerruntime.DefaultSevereThreshold,
		containerruntime.DefaultTimeout,
	)
}

// DeployContainerRuntime deploys the ContainerRuntime custom resources and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration
func (b *Botanist) DeployContainerRuntime(ctx context.Context) error {
	if b.IsRestorePhase() {
		return b.Shoot.Components.Extensions.ContainerRuntime.Restore(ctx, b.Shoot.GetShootState())
	}
	return b.Shoot.Components.Extensions.ContainerRuntime.Deploy(ctx)
}
