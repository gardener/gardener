// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	discoveryv1 "k8s.io/api/discovery/v1"
)

var _ = Describe("EndpointSlices", func() {
	DescribeTable("#GetAddressType",
		func(address string, expected discoveryv1.AddressType) {
			Expect(GetAddressType(address)).To(Equal(expected))
		},
		Entry("IPv4 address", "127.0.0.1", discoveryv1.AddressTypeIPv4),
		Entry("IPv6 address", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", discoveryv1.AddressTypeIPv6),
		Entry("hostname", "example.com", discoveryv1.AddressTypeFQDN),
	)
})
