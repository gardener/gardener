// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/networking/policy/denyall"
)

// DefaultDenyAllTraffic returns a deployer for the DenyAllTraffic.
func (b *Botanist) DefaultDenyAllTraffic() (component.Deployer, error) {
	return denyall.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
	), nil
}
