// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/component"
)

// DeployBootstrapDNSRecord deploys the external DNSRecord pointing to the first control plane machine for bootstrapping
// the self-hosted shoot cluster. The DNSRecord might be publicly resolvable but not publicly accessible, depending on
// shoot's infrastructure provider and network setup. It should be resolvable and accessible from within the shoot
// cluster's network including the machines, so `gardenadm init` can use it to bootstrap the cluster.
func (b *GardenadmBotanist) DeployBootstrapDNSRecord(ctx context.Context) error {
	machine, err := b.GetMachineByIndex(0)
	if err != nil {
		return err
	}

	machineAddr, err := PreferredAddressForMachine(machine)
	if err != nil {
		return err
	}

	b.Shoot.Components.Extensions.ExternalDNSRecord.SetRecordType(extensionsv1alpha1helper.GetDNSRecordType(machineAddr))
	b.Shoot.Components.Extensions.ExternalDNSRecord.SetValues([]string{machineAddr})

	return component.OpWait(b.Shoot.Components.Extensions.ExternalDNSRecord).Deploy(ctx)
}
