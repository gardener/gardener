// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/shared"
)

// DefaultIstioBasicAuthServer returns a deployer for the istio-basic-auth-server.
func (b *Botanist) DefaultIstioBasicAuthServer() (component.DeployWaiter, error) {
	return shared.NewIstioBasicAuthServer(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		b.WantsObservabilityComponents(),
		b.Shoot.GetReplicas(1),
		v1beta1constants.PriorityClassNameShootControlPlane100,
		false,
	)
}
