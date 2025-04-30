// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cidr_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/gardener/gardener/pkg/utils/validation/cidr"
)

var _ = Describe("utils", func() {
	Describe("#ValidateNetworkDisjointedness IPv4", func() {
		var (
			seedPodsCIDR     = "10.241.128.0/17"
			seedServicesCIDR = "10.241.0.0/17"
			seedNodesCIDR    = "10.240.0.0/16"
		)

		It("should pass the validation", func() {
			var (
				podsCIDR     = "10.242.128.0/17"
				servicesCIDR = "10.242.0.0/17"
				nodesCIDR    = "10.243.0.0/16"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				false,
			)

			Expect(errorList).To(BeEmpty())
		})

		It("should fail due to disjointedness", func() {
			var (
				podsCIDR     = seedPodsCIDR
				servicesCIDR = seedServicesCIDR
				nodesCIDR    = seedNodesCIDR
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].nodes"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].services"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].pods"),
			}))))
		})

		It("should pass disjointedness check for non-HA VPN", func() {
			var (
				podsCIDR     = seedPodsCIDR
				servicesCIDR = seedServicesCIDR
				nodesCIDR    = seedNodesCIDR
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				true,
			)

			Expect(errorList).To(BeEmpty())
		})

		It("should fail due to disjointedness of service and pod networks", func() {
			var (
				podsCIDR     = seedServicesCIDR
				servicesCIDR = seedPodsCIDR
				nodesCIDR    = "10.242.128.0/17"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].services"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].pods"),
			}))),
			)
		})

		It("should pass disjointedness check of service and pod networks for non-HA VPN", func() {
			var (
				podsCIDR     = seedServicesCIDR
				servicesCIDR = seedPodsCIDR
				nodesCIDR    = "10.242.128.0/17"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				true,
			)

			Expect(errorList).To(BeEmpty())
		})

		It("should not fail due to missing fields", func() {
			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				nil,
				nil,
				nil,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				false,
			)

			Expect(errorList).To(BeEmpty())
		})

		It("should not fail due to missing fields (HA VPN)", func() {
			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				nil,
				nil,
				nil,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				true,
			)

			Expect(errorList).To(BeEmpty())
		})

		It("should fail due to reserved kube-apiserver mapping range overlap in pod cidr", func() {
			var (
				podsCIDR     = "240.100.0.0/16"
				servicesCIDR = "10.242.0.0/17"
				nodesCIDR    = "10.243.0.0/16"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("[].pods"),
				"Detail": ContainSubstring("pod network intersects with reserved kube-apiserver mapping range"),
			}))))
		})

		It("should fail due to reserved kube-apiserver mapping range overlap in services cidr", func() {
			var (
				podsCIDR     = "10.242.128.0/17"
				servicesCIDR = "240.100.0.0/16"
				nodesCIDR    = "10.243.0.0/16"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("[].services"),
				"Detail": ContainSubstring("service network intersects with reserved kube-apiserver mapping range"),
			}))))
		})

		It("should fail due to reserved kube-apiserver mapping range overlap in nodes cidr", func() {
			var (
				podsCIDR     = "10.242.128.0/17"
				servicesCIDR = "10.242.0.0/17"
				nodesCIDR    = "240.100.0.0/16"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("[].nodes"),
				"Detail": ContainSubstring("node network intersects with reserved kube-apiserver mapping range"),
			}))))
		})

		It("should fail due to reserved 241/8, 242/8 and 243/8 mapping range overlap", func() {
			var (
				podsCIDR     = "241.100.0.0/16"
				servicesCIDR = "242.242.0.0/17"
				nodesCIDR    = "243.243.0.0/16"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				true,
			)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("[].nodes"),
					"Detail": ContainSubstring("shoot node network intersects with reserved shoot service network mapping range (243.0.0.0/8)"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("[].services"),
					"Detail": ContainSubstring("shoot service network intersects with reserved shoot node network mapping range (242.0.0.0/8)"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("[].pods"),
					"Detail": ContainSubstring("shoot pod network intersects with reserved seed pod network mapping range (241.0.0.0/8)"),
				})),
			))
		})

		It("should fail due to reserved 244/8 mapping range overlap", func() {
			var (
				podsCIDR     = "244.242.128.0/17"
				servicesCIDR = "10.242.0.0/17"
				nodesCIDR    = "10.100.0.0/16"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				true,
			)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("[].pods"),
					"Detail": ContainSubstring("shoot pod network intersects with reserved shoot pod network mapping range (244.0.0.0/8)"),
				})),
			))
		})

		It("should fail due to range overlap of seed node network and shoot pod and service network (HA VPN)", func() {
			var (
				podsCIDR     = seedNodesCIDR
				servicesCIDR = seedNodesCIDR
				nodesCIDR    = "10.243.0.0/16"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].pods"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].services"),
			})),
			))
		})

		It("should pass range overlap check of seed node network and shoot pod and service network (non-HA VPN)", func() {
			var (
				podsCIDR     = seedNodesCIDR
				servicesCIDR = seedNodesCIDR
				nodesCIDR    = "10.243.0.0/16"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				true,
			)

			Expect(errorList).To(BeEmpty())
		})

		It("should fail due to seed service network and shoot node network overlap (HA VPN)", func() {
			var (
				podsCIDR     = "10.242.128.0/17"
				servicesCIDR = "10.242.0.0/17"
				nodesCIDR    = "10.241.0.0/17"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].nodes"),
			}))))
		})

		It("should pass overlap check of seed service network and shoot node network (non-HA VPN)", func() {
			var (
				podsCIDR     = "10.242.128.0/17"
				servicesCIDR = "10.242.0.0/17"
				nodesCIDR    = "10.241.0.0/17"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				true,
			)

			Expect(errorList).To(BeEmpty())
		})

		It("should fail due to seed pod network and shoot node network overlap (HA VPN)", func() {
			var (
				podsCIDR     = "10.242.128.0/17"
				servicesCIDR = "10.242.0.0/17"
				nodesCIDR    = seedPodsCIDR
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("[].nodes"),
				"Detail": Equal("shoot node network intersects with seed pod network"),
			}))))
		})

		It("should pass overlap check of seed pod network and shoot node network (non-HA VPN)", func() {
			var (
				podsCIDR     = "10.242.128.0/17"
				servicesCIDR = "10.242.0.0/17"
				nodesCIDR    = seedPodsCIDR
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				true,
			)

			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#ValidateNetworkDisjointedness IPv6", func() {
		var (
			seedPodsCIDRIPv6     = "2001:0db8:65a3::/113"
			seedServicesCIDRIPv6 = "2001:0db8:75a3::/113"
			seedNodesCIDRIPv6    = "2001:0db8:85a3::/112"
		)

		It("should pass the validation", func() {
			var (
				podsCIDR     = "2001:0db8:35a3::/113"
				servicesCIDR = "2001:0db8:45a3::/113"
				nodesCIDR    = "2001:0db8:55a3::/112"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDRIPv6,
				seedPodsCIDRIPv6,
				seedServicesCIDRIPv6,
				false,
			)

			Expect(errorList).To(BeEmpty())
		})

		It("should fail due to disjointedness (HA VPN)", func() {
			var (
				podsCIDR     = seedPodsCIDRIPv6
				servicesCIDR = seedServicesCIDRIPv6
				nodesCIDR    = seedNodesCIDRIPv6
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDRIPv6,
				seedPodsCIDRIPv6,
				seedServicesCIDRIPv6,
				true,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].nodes"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].services"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].pods"),
			}))))
		})

		It("should fail due to disjointedness (non-HA VPN)", func() {
			var (
				podsCIDR     = seedPodsCIDRIPv6
				servicesCIDR = seedServicesCIDRIPv6
				nodesCIDR    = seedNodesCIDRIPv6
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDRIPv6,
				seedPodsCIDRIPv6,
				seedServicesCIDRIPv6,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].nodes"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].services"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].pods"),
			}))))
		})

		It("should fail due to disjointedness of service and pod networks (HA VPN)", func() {
			var (
				podsCIDR     = seedPodsCIDRIPv6
				servicesCIDR = seedServicesCIDRIPv6
				nodesCIDR    = "2001:0db8:55a3::/112"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDRIPv6,
				seedPodsCIDRIPv6,
				seedServicesCIDRIPv6,
				true,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].services"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].pods"),
			}))),
			)
		})

		It("should fail due to disjointedness of service and pod networks (non-HA VPN)", func() {
			var (
				podsCIDR     = seedPodsCIDRIPv6
				servicesCIDR = seedServicesCIDRIPv6
				nodesCIDR    = "2001:0db8:55a3::/112"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDRIPv6,
				seedPodsCIDRIPv6,
				seedServicesCIDRIPv6,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].services"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].pods"),
			}))),
			)
		})

		It("should not fail due to missing fields", func() {
			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				nil,
				nil,
				nil,
				&seedNodesCIDRIPv6,
				seedPodsCIDRIPv6,
				seedServicesCIDRIPv6,
				false,
			)

			Expect(errorList).To(BeEmpty())
		})

		It("should fail due to default vpn range overlap in pod cidr", func() {
			var (
				podsCIDR     = "fd8f:6d53:b97a:1::/120"
				servicesCIDR = "2001:0db8:45a3::/113"
				nodesCIDR    = "2001:0db8:55a3::/112"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDRIPv6,
				seedPodsCIDRIPv6,
				seedServicesCIDRIPv6,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("[].pods"),
				"Detail": ContainSubstring("pod network intersects with default vpn network"),
			}))))
		})

		It("should fail due to default vpn range overlap in services cidr", func() {
			var (
				podsCIDR     = "2001:0db8:35a3::/113"
				servicesCIDR = "fd8f:6d53:b97a:1::/120"
				nodesCIDR    = "2001:0db8:55a3::/112"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDRIPv6,
				seedPodsCIDRIPv6,
				seedServicesCIDRIPv6,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("[].services"),
				"Detail": ContainSubstring("service network intersects with default vpn network"),
			}))))
		})

		It("should fail due to default vpn range overlap in nodes cidr", func() {
			var (
				podsCIDR     = "2001:0db8:35a3::/113"
				servicesCIDR = "2001:0db8:45a3::/113"
				nodesCIDR    = "fd8f:6d53:b97a:1::/120"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDRIPv6,
				seedPodsCIDRIPv6,
				seedServicesCIDRIPv6,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("[].nodes"),
				"Detail": ContainSubstring("node network intersects with default vpn network"),
			}))))
		})

		It("should fail due to range overlap of seed node network and shoot pod and service network (HA VPN)", func() {
			var (
				podsCIDR     = seedNodesCIDRIPv6
				servicesCIDR = seedNodesCIDRIPv6
				nodesCIDR    = "2001:0db8:55a3::/112"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDRIPv6,
				seedPodsCIDRIPv6,
				seedServicesCIDRIPv6,
				true,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].pods"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].services"),
			})),
			))
		})

		It("should fail due to range overlap of seed node network and shoot pod and service network (non-HA VPN)", func() {
			var (
				podsCIDR     = seedNodesCIDRIPv6
				servicesCIDR = seedNodesCIDRIPv6
				nodesCIDR    = "2001:0db8:55a3::/112"
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDRIPv6,
				seedPodsCIDRIPv6,
				seedServicesCIDRIPv6,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].pods"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].services"),
			})),
			))
		})

		It("should fail due to seed service network and shoot node network overlap (HA VPN)", func() {
			var (
				podsCIDR     = "2001:0db8:35a3::/113"
				servicesCIDR = "2001:0db8:45a3::/113"
				nodesCIDR    = seedServicesCIDRIPv6
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDRIPv6,
				seedPodsCIDRIPv6,
				seedServicesCIDRIPv6,
				true,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].nodes"),
			}))))
		})

		It("should fail due to seed service network and shoot node network overlap (non-HA VPN)", func() {
			var (
				podsCIDR     = "2001:0db8:35a3::/113"
				servicesCIDR = "2001:0db8:45a3::/113"
				nodesCIDR    = seedServicesCIDRIPv6
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDRIPv6,
				seedPodsCIDRIPv6,
				seedServicesCIDRIPv6,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].nodes"),
			}))))
		})

		It("should fail due to seed pod network and shoot node network overlap (HA VPN)", func() {
			var (
				podsCIDR     = "2001:0db8:35a3::/113"
				servicesCIDR = "2001:0db8:45a3::/113"
				nodesCIDR    = seedPodsCIDRIPv6
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDRIPv6,
				seedPodsCIDRIPv6,
				seedServicesCIDRIPv6,
				true,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("[].nodes"),
				"Detail": Equal("shoot node network intersects with seed pod network"),
			}))))
		})

		It("should fail due to seed pod network and shoot node network overlap (non-HA VPN)", func() {
			var (
				podsCIDR     = "2001:0db8:35a3::/113"
				servicesCIDR = "2001:0db8:45a3::/113"
				nodesCIDR    = seedPodsCIDRIPv6
			)

			errorList := ValidateNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				&seedNodesCIDRIPv6,
				seedPodsCIDRIPv6,
				seedServicesCIDRIPv6,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("[].nodes"),
				"Detail": Equal("shoot node network intersects with seed pod network"),
			}))))
		})
	})

	Describe("#ValidateMultiNetworkDisjointedness", func() {
		var (
			seedPodsCIDR     = "10.241.128.0/17"
			seedServicesCIDR = "10.241.0.0/17"
			seedNodesCIDR    = "10.240.0.0/16"
		)

		It("should pass the validation", func() {
			var (
				podsCIDRs     = []string{"10.242.128.0/17", "2001:db8:1::/64"}
				servicesCIDRs = []string{"10.242.0.0/17", "2001:db8:2::/64"}
				nodesCIDRs    = []string{"10.243.0.0/16", "2001:db8:3::/64"}
			)

			errorList := ValidateMultiNetworkDisjointedness(
				field.NewPath(""),
				nodesCIDRs,
				podsCIDRs,
				servicesCIDRs,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				false,
				true,
			)

			Expect(errorList).To(BeEmpty())
		})

		It("should fail due to disjointedness (HA VPN)", func() {
			var (
				podsCIDR     = []string{seedPodsCIDR}
				servicesCIDR = []string{seedServicesCIDR}
				nodesCIDR    = []string{seedNodesCIDR}
			)

			errorList := ValidateMultiNetworkDisjointedness(
				field.NewPath(""),
				nodesCIDR,
				podsCIDR,
				servicesCIDR,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				false,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].nodes"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].services"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].pods"),
			}))))
		})

		It("should pass disjointedness check (non-HA VPN)", func() {
			var (
				podsCIDR     = []string{seedPodsCIDR}
				servicesCIDR = []string{seedServicesCIDR}
				nodesCIDR    = []string{seedNodesCIDR}
			)

			errorList := ValidateMultiNetworkDisjointedness(
				field.NewPath(""),
				nodesCIDR,
				podsCIDR,
				servicesCIDR,
				&seedNodesCIDR,
				seedPodsCIDR,
				seedServicesCIDR,
				false,
				true,
			)

			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#ValidateShootNetworkDisjointedness IPv4", func() {
		It("should pass the validation", func() {
			var (
				podsCIDR     = "10.242.128.0/17"
				servicesCIDR = "10.242.0.0/17"
				nodesCIDR    = "10.241.0.0/16"
			)

			errorList := ValidateShootNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				false,
			)

			Expect(errorList).To(BeEmpty())
		})

		It("should fail due to missing fields", func() {
			errorList := ValidateShootNetworkDisjointedness(
				field.NewPath(""),
				nil,
				nil,
				nil,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("[].services"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("[].pods"),
			})),
			))
		})

		It("should fail due to disjointedness of node, service and pod networks", func() {
			var (
				nodesCIDR    = "10.241.0.0/16"
				podsCIDR     = nodesCIDR
				servicesCIDR = nodesCIDR
			)

			errorList := ValidateShootNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].services"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].pods"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].services"),
			})),
			))
		})
	})

	Describe("#ValidateShootNetworkDisjointedness IPv6", func() {
		It("should pass the validation", func() {
			var (
				podsCIDR     = "2001:0db8:35a3::/113"
				servicesCIDR = "2001:0db8:45a3::/113"
				nodesCIDR    = "2001:0db8:55a3::/112"
			)

			errorList := ValidateShootNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				false,
			)

			Expect(errorList).To(BeEmpty())
		})

		It("should fail due to missing fields", func() {
			errorList := ValidateShootNetworkDisjointedness(
				field.NewPath(""),
				nil,
				nil,
				nil,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("[].services"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("[].pods"),
			})),
			))
		})

		It("should fail due to disjointedness of node, service and pod networks", func() {
			var (
				nodesCIDR    = "2001:0db8:55a3::/112"
				podsCIDR     = nodesCIDR
				servicesCIDR = nodesCIDR
			)

			errorList := ValidateShootNetworkDisjointedness(
				field.NewPath(""),
				&nodesCIDR,
				&podsCIDR,
				&servicesCIDR,
				false,
			)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].services"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].pods"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].services"),
			})),
			))
		})
	})
})
