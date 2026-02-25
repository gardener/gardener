// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/shared"
)

// DefaultExtAuthzServer returns a deployer for the external authorization server.
func (b *Botanist) DefaultExtAuthzServer() (component.DeployWaiter, error) {
	return shared.NewExtAuthzServer(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		true,
		b.Shoot.GetReplicas(1),
		v1beta1constants.PriorityClassNameShootControlPlane100,
		false,
	)
}

// DeployExtAuthzServer deploys the external authorization server in the Seed cluster.
func (b *Botanist) DeployExtAuthzServer(ctx context.Context) error {
	// Disable external authorization server if no observability components are needed
	if !b.WantsObservabilityComponents() {
		return b.Shoot.Components.ControlPlane.ExtAuthzServer.Destroy(ctx)
	}

	return b.Shoot.Components.ControlPlane.ExtAuthzServer.Deploy(ctx)
}
