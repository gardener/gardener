// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/api/extensions/v1alpha1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
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

	machineAddr, err := PreferredNodeAddress(machine.Status.Addresses)
	if err != nil {
		return fmt.Errorf("failed getting preferred address for machine %q: %w", machine.Name, err)
	}

	b.Shoot.Components.Extensions.ExternalDNSRecord.SetRecordType(extensionsv1alpha1helper.GetDNSRecordType(machineAddr))
	b.Shoot.Components.Extensions.ExternalDNSRecord.SetValues([]string{machineAddr})

	return component.OpWait(b.Shoot.Components.Extensions.ExternalDNSRecord).Deploy(ctx)
}

// RestoreExternalDNSRecord restores the external DNSRecord for the self-hosted shoot's API server.
// For extension-based exposure the DNS target is set to the ingress that is read from the
// SelfHostedShootExposure status.
// For DNS-based exposure the preferred address of each control-plane node is used directly.
// For dual-stack setups, IP families are preferred in the order they appear in spec.networking.ipFamilies.
func (b *GardenadmBotanist) RestoreExternalDNSRecord(ctx context.Context) error {
	addresses, recordType, err := b.externalDNSRecordValues(ctx)
	if err != nil {
		return err
	}

	b.Shoot.Components.Extensions.ExternalDNSRecord.SetRecordType(recordType)
	b.Shoot.Components.Extensions.ExternalDNSRecord.SetValues(addresses)

	if err := b.Shoot.Components.Extensions.ExternalDNSRecord.Restore(ctx, b.Shoot.GetShootState()); err != nil {
		return err
	}
	return b.Shoot.Components.Extensions.ExternalDNSRecord.Wait(ctx)
}

func (b *GardenadmBotanist) externalDNSRecordValues(ctx context.Context) ([]string, extensionsv1alpha1.DNSRecordType, error) {
	ipFamilies := b.Shoot.GetInfo().Spec.Networking.IPFamilies

	if b.Shoot.HasExtensionExposure() {
		return extensionsv1alpha1helper.DNSValuesFromIngress(b.Shoot.Components.Extensions.SelfHostedShootExposure.Ingress, ipFamilies)
	}

	nodes, err := b.ListControlPlaneNodes(ctx)
	if err != nil {
		return nil, "", err
	}
	return extensionsv1alpha1helper.DNSValuesFromNodes(nodes, ipFamilies, corev1.NodeInternalIP, corev1.NodeExternalIP)
}
