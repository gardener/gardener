// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/nodemanagement/dependencywatchdog"
)

// DefaultDependencyWatchdogAccess returns an instance of the Deployer which reconciles the resources so that DependencyWatchdogAccess can access a
// shoot cluster.
func (b *Botanist) DefaultDependencyWatchdogAccess() component.Deployer {
	return dependencywatchdog.NewAccess(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		dependencywatchdog.AccessValues{
			ServerInCluster: b.Shoot.ComputeInClusterAPIServerAddress(false),
		},
	)
}

// DeployDependencyWatchdogAccess deploys the DependencyWatchdogAccess resources.
func (b *Botanist) DeployDependencyWatchdogAccess(ctx context.Context) error {
	if !v1beta1helper.SeedSettingDependencyWatchdogProberEnabled(b.Seed.GetInfo().Spec.Settings) {
		return b.Shoot.Components.DependencyWatchdogAccess.Destroy(ctx)
	}

	return b.Shoot.Components.DependencyWatchdogAccess.Deploy(ctx)
}
