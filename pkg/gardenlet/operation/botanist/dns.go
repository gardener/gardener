// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
)

const (
	// DNSRecordSecretPrefix is a constant for prefixing secrets referenced by DNSRecords
	DNSRecordSecretPrefix = "dnsrecord"
)

// NeedsExternalDNS returns true if the Shoot cluster needs external DNS.
func (b *Botanist) NeedsExternalDNS() bool {
	return b.Shoot.GetInfo().Spec.DNS != nil &&
		b.Shoot.GetInfo().Spec.DNS.Domain != nil &&
		b.Shoot.ExternalClusterDomain != nil &&
		b.Shoot.ExternalDomain != nil &&
		b.Shoot.ExternalDomain.Provider != "unmanaged"
}

// NeedsInternalDNS returns true if the Shoot cluster needs internal DNS.
func (b *Botanist) NeedsInternalDNS() bool {
	return b.Garden != nil &&
		b.Garden.InternalDomain != nil &&
		b.Garden.InternalDomain.Provider != "unmanaged"
}

func (b *Botanist) newDNSComponentsTargetingAPIServerAddress() {
	if b.NeedsInternalDNS() {
		b.Shoot.Components.Extensions.InternalDNSRecord.SetRecordType(extensionsv1alpha1helper.GetDNSRecordType(b.APIServerAddress))
		b.Shoot.Components.Extensions.InternalDNSRecord.SetValues([]string{b.APIServerAddress})
	}

	if b.NeedsExternalDNS() {
		b.Shoot.Components.Extensions.ExternalDNSRecord.SetRecordType(extensionsv1alpha1helper.GetDNSRecordType(b.APIServerAddress))
		b.Shoot.Components.Extensions.ExternalDNSRecord.SetValues([]string{b.APIServerAddress})
	}
}
