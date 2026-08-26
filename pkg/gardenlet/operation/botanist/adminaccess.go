// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/kubernetes/adminaccess"
)

// DefaultAdminAccess returns an instance of the Deployer which reconciles the resources so that a shoot access secret
// with cluster-admin access gets created
func (b *Botanist) DefaultAdminAccess() component.Deployer {
	return adminaccess.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
	)
}
