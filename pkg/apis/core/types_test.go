// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/apis/core"
)

var _ = Describe("API Types", func() {
	Describe("#IsIPv4SingleStack", func() {
		It("should return true for empty IP families", func() {
			Expect(IsIPv4SingleStack(nil)).To(BeTrue())
		})
		It("should return true for IPv4 single-stack", func() {
			Expect(IsIPv4SingleStack([]IPFamily{IPFamilyIPv4})).To(BeTrue())
		})
		It("should return false for dual-stack", func() {
			Expect(IsIPv4SingleStack([]IPFamily{IPFamilyIPv4, IPFamilyIPv6})).To(BeFalse())
			Expect(IsIPv4SingleStack([]IPFamily{IPFamilyIPv6, IPFamilyIPv4})).To(BeFalse())
		})
		It("should return false for IPv6 single-stack", func() {
			Expect(IsIPv4SingleStack([]IPFamily{IPFamilyIPv6})).To(BeFalse())
		})
	})

	Describe("#IsIPv6SingleStack", func() {
		It("should return false for empty IP families", func() {
			Expect(IsIPv6SingleStack(nil)).To(BeFalse())
		})
		It("should return false for IPv4 single-stack", func() {
			Expect(IsIPv6SingleStack([]IPFamily{IPFamilyIPv4})).To(BeFalse())
		})
		It("should return false for dual-stack", func() {
			Expect(IsIPv6SingleStack([]IPFamily{IPFamilyIPv4, IPFamilyIPv6})).To(BeFalse())
			Expect(IsIPv6SingleStack([]IPFamily{IPFamilyIPv6, IPFamilyIPv4})).To(BeFalse())
		})
		It("should return true for IPv6 single-stack", func() {
			Expect(IsIPv6SingleStack([]IPFamily{IPFamilyIPv6})).To(BeTrue())
		})
	})
})
