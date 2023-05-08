package net

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
		Expect(GetBitLen(ip)).To(Equal(32))
	})
	It("should fail parsing IPv6 address and return the default 32", func() {
		ip := "XYZ:db8:3::"
		Expect(GetBitLen(ip)).To(Equal(32))
	})
})
