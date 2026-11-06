// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/api/extensions/v1alpha1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
)

// DeployExternalDNSRecord deploys the external DNSRecord for the self-hosted shoot's API server.
// For extension-based exposure the DNS target is set to the ingress that is read from the
// SelfHostedShootExposure status.
// For DNS-based exposure the preferred address of each control-plane node is used directly.
// For dual-stack setups, IP families are preferred in the order they appear in spec.networking.ipFamilies.
func (b *GardenadmBotanist) DeployExternalDNSRecord(ctx context.Context) error {
	addresses, recordType, err := b.externalDNSRecordValues(ctx)
	if err != nil {
		return err
	}

	b.Shoot.Components.Extensions.ExternalDNSRecord.SetRecordType(recordType)
	b.Shoot.Components.Extensions.ExternalDNSRecord.SetValues(addresses)

	return component.OpWait(b.Shoot.Components.Extensions.ExternalDNSRecord).Deploy(ctx)
}

// DeployInternalDNSRecord deploys the internal DNSRecord for the self-hosted shoot's API server. It points to the
// internal IPs of the control plane nodes, so that components on the worker nodes can talk to the control plane without
// depending on the control plane exposure.
// For dual-stack setups, IP families are preferred in the order they appear in spec.networking.ipFamilies.
func (b *GardenadmBotanist) DeployInternalDNSRecord(ctx context.Context) error {
	nodes, err := b.ListControlPlaneNodes(ctx)
	if err != nil {
		return err
	}

	addresses, recordType, err := extensionsv1alpha1helper.DNSValuesFromNodes(nodes, b.Shoot.GetInfo().Spec.Networking.IPFamilies, corev1.NodeInternalIP)
	if err != nil {
		return err
	}

	b.Shoot.Components.Extensions.InternalDNSRecord.SetRecordType(recordType)
	b.Shoot.Components.Extensions.InternalDNSRecord.SetValues(addresses)

	return component.OpWait(b.Shoot.Components.Extensions.InternalDNSRecord).Deploy(ctx)
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
