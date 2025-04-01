// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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

	Describe("CloudProfile", func() {
		Describe("CapabilityValues", func() {
			var values CapabilityValues

			BeforeEach(func() {
				values = []string{"a", "b", "c"}
			})

			Describe("#Contains", func() {
				It("should return true if both contain the same values", func() {
					Expect(values.Contains("a", "b", "c")).To(BeTrue())
				})

				It("should return true for a subset", func() {
					Expect(values.Contains("a", "b")).To(BeTrue())
				})

				It("should return false if not all values are contained", func() {
					Expect(values.Contains("a", "d")).To(BeFalse())
				})
			})

			Describe("#IsSubsetOf", func() {
				It("should return true if it is a subset", func() {
					Expect(values.IsSubsetOf([]string{"a", "b", "c", "d"})).To(BeTrue())
				})

				It("should return true if both contain the same values", func() {
					Expect(values.IsSubsetOf([]string{"a", "b", "c"})).To(BeTrue())
				})

				It("should return false if it is not a subset", func() {
					Expect(values.IsSubsetOf([]string{"a", "b"})).To(BeFalse())
				})
			})
		})

		Describe("Capabilities", func() {
			Describe("#len", func() {
				It("should return true if it has capabilities defined", func() {
					Expect(Capabilities{"fooCap": []string{"a", "b", "c"}}).NotTo(BeEmpty())
				})

				It("should return false if it hasn't capabilities defined", func() {
					Expect(Capabilities{}).To(BeEmpty())
				})
			})
		})
	})
})
