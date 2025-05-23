// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package net_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/net"
)

var _ = Describe("#GetBitLen", func() {
	It("should parse IPv4 address correctly", func() {
		ip := "10.10.0.26"
		Expect(GetBitLen(ip)).To(Equal(32))
	})
	It("should parse IPv6 address correctly", func() {
		ip := "2002:db8:3::"
		Expect(GetBitLen(ip)).To(Equal(128))
	})
	It("should fail parsing IPv4 address and return the default 32", func() {
		ip := "500.500.500.123"
		bitLen, err := GetBitLen(ip)
		Expect(err).To(HaveOccurred())
		Expect(bitLen).To(Equal(0))
	})
	It("should fail parsing IPv6 address and return the default 32", func() {
		ip := "XYZ:db8:3::"
		bitLen, err := GetBitLen(ip)
		Expect(err).To(HaveOccurred())
		Expect(bitLen).To(Equal(0))
	})
})
