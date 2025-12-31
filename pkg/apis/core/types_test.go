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

	Describe("#IsDualStack", func() {
		It("should return false for empty IP families", func() {
			Expect(IsDualStack(nil)).To(BeFalse())
		})
		It("should return false for IPv4 single-stack", func() {
			Expect(IsDualStack([]IPFamily{IPFamilyIPv4})).To(BeFalse())
		})
		It("should return false for IPv6 single-stack", func() {
			Expect(IsDualStack([]IPFamily{IPFamilyIPv6})).To(BeFalse())
		})
		It("should return true for dual-stack in IPv4, IPv6 order", func() {
			Expect(IsDualStack([]IPFamily{IPFamilyIPv4, IPFamilyIPv6})).To(BeTrue())
		})
		It("should return true for dual-stack in IPv6, IPv4 order", func() {
			Expect(IsDualStack([]IPFamily{IPFamilyIPv6, IPFamilyIPv4})).To(BeTrue())
		})
	})

	DescribeTable("#VersionClassification.IsActive", func(v VersionClassification, want bool) {
		Expect(v.IsActive()).To(Equal(want))
	},
		Entry("#ClassificationUnavailable is not active", ClassificationUnavailable, false),
		Entry("#ClassificationPreview is active", ClassificationPreview, true),
		Entry("#ClassificationSupported is active", ClassificationSupported, true),
		Entry("#ClassificationDeprecated is active", ClassificationDeprecated, true),
		Entry("#ClassificationExpired is not active", ClassificationExpired, false),
	)
	Describe("#GetCapabilitiesWithAppliedDefaults", func() {
		It("should apply default values when capabilities are nil", func() {
			var capabilities Capabilities
			capabilityDefinitions := []CapabilityDefinition{
				{Name: "capability1", Values: []string{"value1", "value2"}},
				{Name: "architecture", Values: []string{"amd64"}},
			}

			result := GetCapabilitiesWithAppliedDefaults(capabilities, capabilityDefinitions)

			Expect(result).To(Equal(Capabilities{
				"capability1":  []string{"value1", "value2"},
				"architecture": []string{"amd64"},
			}))
		})

		It("should retain existing values and apply defaults for missing capabilities", func() {
			capabilities := Capabilities{
				"capability1": []string{"value1"},
			}
			capabilityDefinitions := []CapabilityDefinition{
				{Name: "capability1", Values: []string{"value1", "value2"}},
				{Name: "architecture", Values: []string{"amd64"}},
			}

			result := GetCapabilitiesWithAppliedDefaults(capabilities, capabilityDefinitions)

			Expect(result).To(Equal(Capabilities{
				"capability1":  []string{"value1"},
				"architecture": []string{"amd64"},
			}))
		})
	})
})
