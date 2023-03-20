// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package botanist

import (
	"strings"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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
		!strings.HasSuffix(*b.Shoot.ExternalClusterDomain, ".nip.io") &&
		b.Shoot.ExternalDomain != nil &&
		b.Shoot.ExternalDomain.Provider != "unmanaged"
}

// NeedsInternalDNS returns true if the Shoot cluster needs internal DNS.
func (b *Botanist) NeedsInternalDNS() bool {
	return b.Garden.InternalDomain != nil &&
		b.Garden.InternalDomain.Provider != "unmanaged"
}

// APIServerSNIPodMutatorEnabled returns false if the value of the Shoot annotation
// 'alpha.featuregates.shoot.gardener.cloud/apiserver-sni-pod-injector' is 'disable' or
// APIServerSNI feature is disabled.
func (b *Botanist) APIServerSNIPodMutatorEnabled() bool {
	sniEnabled := b.APIServerSNIEnabled()
	if !sniEnabled {
		return false
	}

	vs, ok := b.Shoot.GetInfo().GetAnnotations()[v1beta1constants.AnnotationShootAPIServerSNIPodInjector]
	if !ok {
		return true
	}

	return vs != v1beta1constants.AnnotationShootAPIServerSNIPodInjectorDisableValue
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
