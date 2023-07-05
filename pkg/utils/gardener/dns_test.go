// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"

	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Dns", func() {
	DescribeTable("#GetDomainInfoFromAnnotations",
		func(annotations map[string]string, expectedProvider, expectedDomain, expectedZone, expectedErr gomegatypes.GomegaMatcher) {
			provider, domain, zone, err := GetDomainInfoFromAnnotations(annotations)
			Expect(provider).To(expectedProvider)
			Expect(domain).To(expectedDomain)
			Expect(zone).To(expectedZone)
			Expect(err).To(expectedErr)
		},

		Entry("no annotations", nil, BeEmpty(), BeEmpty(), BeEmpty(), HaveOccurred()),
		Entry("no domain", map[string]string{
			DNSProvider: "bar",
		}, BeEmpty(), BeEmpty(), BeEmpty(), HaveOccurred()),
		Entry("no provider", map[string]string{
			DNSDomain: "foo",
		}, BeEmpty(), BeEmpty(), BeEmpty(), HaveOccurred()),
		Entry("all present", map[string]string{
			DNSProvider: "bar",
			DNSDomain:   "foo",
			DNSZone:     "zoo",
		}, Equal("bar"), Equal("foo"), Equal("zoo"), Not(HaveOccurred())),
	)

	DescribeTable("#GenerateDNSProviderName",
		func(secretName, providerType, expectedName string) {
			Expect(GenerateDNSProviderName(secretName, providerType)).To(Equal(expectedName))
		},

		Entry("both empty", "", "", ""),
		Entry("secretName empty", "", "provider-type", "provider-type"),
		Entry("providerType empty", "secret-name", "", "secret-name"),
		Entry("both set", "secret-name", "provider-type", "provider-type-secret-name"),
	)
})
