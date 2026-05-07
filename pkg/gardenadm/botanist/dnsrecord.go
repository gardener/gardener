// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"slices"

	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/api/extensions/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
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

	machineAddr, err := PreferredAddress(machine.Status.Addresses)
	if err != nil {
		return fmt.Errorf("failed getting preferred address for machine %q: %w", machine.Name, err)
	}

	b.Shoot.Components.Extensions.ExternalDNSRecord.SetRecordType(extensionsv1alpha1helper.GetDNSRecordType(machineAddr))
	b.Shoot.Components.Extensions.ExternalDNSRecord.SetValues([]string{machineAddr})

	return component.OpWait(b.Shoot.Components.Extensions.ExternalDNSRecord).Deploy(ctx)
}

// RestoreExternalDNSRecord restores the external DNSRecord for the self-hosted shoot's API server.
// For extension-based exposure the DNS target is set to the LoadBalancer ingress that is read
// from the SelfHostedShootExposure status.
// For DNS-based exposure the preferred address of each control-plane node is used directly.
// For dual-stack LBs, IP families are preferred in the order they appear in spec.networking.ipFamilies.
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
	if b.Shoot.HasExtensionExposure() {
		return b.extensionExposureDNSRecordValues()
	}
	return b.nodeAddressDNSRecordValues(ctx)
}

func (b *GardenadmBotanist) extensionExposureDNSRecordValues() ([]string, extensionsv1alpha1.DNSRecordType, error) {
	ingress := b.Shoot.Components.Extensions.SelfHostedShootExposure.GetIngress()
	if len(ingress) == 0 {
		return nil, "", fmt.Errorf("SelfHostedShootExposure has no ingress yet")
	}
	// Prefer IPs over hostnames
	var ips, hostnames []string
	for _, i := range ingress {
		if i.IP != "" {
			ips = append(ips, i.IP)
			continue
		}
		if i.Hostname != "" {
			hostnames = append(hostnames, i.Hostname)
		}
	}
	switch {
	case len(ips) > 0:
		return filterByIPFamily(ips, b.Shoot.GetInfo().Spec.Networking.IPFamilies,
			fmt.Errorf("LoadBalancer ingress IPs %v do not match any configured IP family", ips))
	case len(hostnames) > 0:
		return hostnames, extensionsv1alpha1.DNSRecordTypeCNAME, nil
	default:
		return nil, "", fmt.Errorf("LoadBalancer ingress has neither IP nor hostname")
	}
}

func (b *GardenadmBotanist) nodeAddressDNSRecordValues(ctx context.Context) ([]string, extensionsv1alpha1.DNSRecordType, error) {
	nodes, err := b.listControlPlaneNodes(ctx)
	if err != nil {
		return nil, "", err
	}

	addresses := make([]string, 0, len(nodes))
	for _, node := range nodes {
		addr, err := PreferredAddress(node.Status.Addresses)
		if err != nil {
			return nil, "", fmt.Errorf("failed getting preferred address for node %q: %w", node.Name, err)
		}
		addresses = append(addresses, addr)
	}

	return filterByIPFamily(addresses, b.Shoot.GetInfo().Spec.Networking.IPFamilies,
		fmt.Errorf("control plane node addresses %v do not match any configured IP family", addresses))
}

// filterByIPFamily returns the subset of addresses matching the first IP family in ipFamilies
// that has at least one match, along with its DNS record type. noneMatchErr is returned if no family matches.
func filterByIPFamily(addresses []string, ipFamilies []gardencorev1beta1.IPFamily, noneMatchErr error) ([]string, extensionsv1alpha1.DNSRecordType, error) {
	for _, family := range ipFamilies {
		recordType := ipFamilyToDNSRecordType(family)
		filtered := slices.DeleteFunc(slices.Clone(addresses), func(addr string) bool {
			return extensionsv1alpha1helper.GetDNSRecordType(addr) != recordType
		})
		if len(filtered) > 0 {
			return filtered, recordType, nil
		}
	}
	return nil, "", noneMatchErr
}

func ipFamilyToDNSRecordType(family gardencorev1beta1.IPFamily) extensionsv1alpha1.DNSRecordType {
	if family == gardencorev1beta1.IPFamilyIPv6 {
		return extensionsv1alpha1.DNSRecordTypeAAAA
	}
	return extensionsv1alpha1.DNSRecordTypeA
}
