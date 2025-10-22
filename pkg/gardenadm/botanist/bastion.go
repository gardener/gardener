// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"github.com/gardener/gardener/pkg/component/extensions/bastion"
)

// DefaultBastion creates the Bastion component for accessing the control plane machines in `gardenadm bootstrap`.
func (b *GardenadmBotanist) DefaultBastion() *bastion.Bastion {
	return bastion.New(b.Logger, b.SeedClientSet.Client(), b.SecretsManager, &bastion.Values{
		Name:      "gardenadm-bootstrap",
		Namespace: b.Shoot.ControlPlaneNamespace,
		Provider:  b.Shoot.GetInfo().Spec.Provider.Type,
	})
}
