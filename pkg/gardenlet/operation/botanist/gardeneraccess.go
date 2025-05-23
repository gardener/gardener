// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"github.com/gardener/gardener/pkg/component"
	gardeneraccess "github.com/gardener/gardener/pkg/component/gardener/access"
)

// DefaultGardenerAccess returns an instance of the Deployer which reconciles the resources so that GardenerAccess can access a
// shoot cluster.
func (b *Botanist) DefaultGardenerAccess() component.Deployer {
	return gardeneraccess.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		gardeneraccess.Values{
			ServerInCluster:    b.Shoot.ComputeInClusterAPIServerAddress(false),
			ServerOutOfCluster: b.Shoot.ComputeOutOfClusterAPIServerAddress(true),
		},
	)
}
