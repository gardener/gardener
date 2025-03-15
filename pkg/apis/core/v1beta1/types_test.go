// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("#IsIPv4SingleStack", func() {
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

var _ = Describe("#IsIPv6SingleStack", func() {
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

var _ = Describe("#Unmarshal CapabilitiesValues", func() {
	It("should sanitize capability values on UnmarshalJSON", func() {
		var capabilities2 = Capabilities{}

		values1 := CapabilityValues{}
		_ = values1.UnmarshalJSON([]byte(`"amd64,arm64,amd32,  asap   ,   I look weird"`))
		values2 := CapabilityValues{}
		_ = values2.UnmarshalJSON([]byte(`"gen1,gen4"`))

		capabilities2["architecture"] = values1
		capabilities2["hypervisorType"] = values2

		architectureValues := capabilities2["architecture"]
		Expect(architectureValues.Values).To(ConsistOf("amd64", "arm64", "amd32", "asap", "I look weird"))
		hypervisorValues := capabilities2["hypervisorType"]
		Expect(hypervisorValues.Values).To(ConsistOf("gen1", "gen4"))
	})
})
