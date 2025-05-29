// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/validation"
)

var _ = Describe("Network validation tests", func() {
	var network *extensionsv1alpha1.Network

	BeforeEach(func() {
		network = &extensionsv1alpha1.Network{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-network",
				Namespace: "test-namespace",
			},
			Spec: extensionsv1alpha1.NetworkSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type:           "provider",
					ProviderConfig: &runtime.RawExtension{},
				},
				PodCIDR:     "10.20.30.40/26",
				ServiceCIDR: "10.30.40.50/26",
			},
		}
	})

	Describe("#ValidateNetwork", func() {
		It("should forbid empty Network resources", func() {
			errorList := ValidateNetwork(&extensionsv1alpha1.Network{})

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.namespace"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.type"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.podCIDR"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.serviceCIDR"),
			}))))
		})

		Context("IPv4", func() {
			It("should allow valid network resources", func() {
				errorList := ValidateNetwork(network)
				Expect(errorList).To(BeEmpty())
			})

			It("should forbid IPv6 CIDRs with no IP family specified", func() {
				network.Spec.PodCIDR = "2001:db8:1::/48"
				network.Spec.ServiceCIDR = "2001:db8:3::/48"
				network.Spec.IPFamilies = nil

				errorList := ValidateNetwork(network)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.podCIDR"),
					"Detail": Equal("must be a valid IPv4 address"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.serviceCIDR"),
					"Detail": Equal("must be a valid IPv4 address"),
				}))))
			})

			It("should forbid IPv6 CIDRs with IPv4 IP family", func() {
				network.Spec.PodCIDR = "2001:db8:1::/48"
				network.Spec.ServiceCIDR = "2001:db8:3::/48"
				network.Spec.IPFamilies = []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4}

				errorList := ValidateNetwork(network)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.podCIDR"),
					"Detail": Equal("must be a valid IPv4 address"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.serviceCIDR"),
					"Detail": Equal("must be a valid IPv4 address"),
				}))))
			})

			It("should forbid Network with invalid CIDRs", func() {
				network.Spec.PodCIDR = "this-is-no-cidr"
				network.Spec.ServiceCIDR = "this-is-still-no-cidr"

				errorList := ValidateNetwork(network)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.podCIDR"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.serviceCIDR"),
				}))))
			})

			It("should forbid Network with overlapping pod and service CIDRs", func() {
				network.Spec.PodCIDR = network.Spec.ServiceCIDR

				errorList := ValidateNetwork(network)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.serviceCIDR"),
				}))))
			})
		})

		Context("IPv6", func() {
			BeforeEach(func() {
				network.Spec.IPFamilies = []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv6}
			})

			It("should allow valid network resources", func() {
				network.Spec.PodCIDR = "2001:db8:1::/48"
				network.Spec.ServiceCIDR = "2001:db8:3::/48"

				errorList := ValidateNetwork(network)
				Expect(errorList).To(BeEmpty())
			})

			It("should forbid IPv4 CIDRs with IPv6 IP family", func() {
				errorList := ValidateNetwork(network)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.podCIDR"),
					"Detail": Equal("must be a valid IPv6 address"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.serviceCIDR"),
					"Detail": Equal("must be a valid IPv6 address"),
				}))))
			})

			It("should forbid Network with invalid CIDRs", func() {
				network.Spec.ServiceCIDR = "2001:db/###8:3::/48"
				network.Spec.PodCIDR = "2003:db/###8:3::/48"

				errorList := ValidateNetwork(network)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.podCIDR"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.serviceCIDR"),
				}))))
			})

			It("should allow Network with overlapping pod and service CIDRs", func() {
				network.Spec.ServiceCIDR = "2001:db8:3::/48"
				network.Spec.PodCIDR = network.Spec.ServiceCIDR

				errorList := ValidateNetwork(network)
				Expect(errorList).To(BeEmpty())
			})
		})

		Context("dual-stack", func() {
			BeforeEach(func() {
				network.Spec.IPFamilies = []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv6, extensionsv1alpha1.IPFamilyIPv4}
			})

			It("should allow valid network resources", func() {
				network.Spec.PodCIDR = "10.20.30.40/26"
				network.Spec.ServiceCIDR = "10.30.40.50/26"

				errorList := ValidateNetwork(network)
				Expect(errorList).To(BeEmpty())
			})
		})
	})

	Describe("#ValidateNetworkUpdate", func() {
		It("should prevent updating anything if deletion time stamp is set", func() {
			now := metav1.Now()
			network.DeletionTimestamp = &now

			newNetwork := prepareNetworkForUpdate(network)
			newNetwork.DeletionTimestamp = &now
			newNetwork.Spec.ProviderConfig = nil

			errorList := ValidateNetworkUpdate(newNetwork, network)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec"),
				"Detail": Equal("cannot update network spec if deletion timestamp is set. Requested changes: DefaultSpec.ProviderConfig: <nil pointer> != runtime.RawExtension"),
			}))))
		})

		It("should prevent updating the type or the cidrs", func() {
			newNetwork := prepareNetworkForUpdate(network)
			newNetwork.Spec.Type = "changed-type"
			newNetwork.Spec.PodCIDR = "10.21.30.40/26"
			newNetwork.Spec.ServiceCIDR = "10.31.40.50/26"

			errorList := ValidateNetworkUpdate(newNetwork, network)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.type"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.podCIDR"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.serviceCIDR"),
			}))))
		})

		It("should allow updating the provider config", func() {
			newNetwork := prepareNetworkForUpdate(network)
			newNetwork.Spec.ProviderConfig = nil

			errorList := ValidateNetworkUpdate(newNetwork, network)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow updating ipFamilies from IPv4 to dual-stack [IPv4, IPv6]", func() {
			network.Spec.IPFamilies = []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4}
			newNetwork := prepareNetworkForUpdate(network)
			newNetwork.Spec.IPFamilies = []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4, extensionsv1alpha1.IPFamilyIPv6}

			errorList := ValidateNetworkUpdate(newNetwork, network)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow updating ipFamilies from IPv6 to dual-stack [IPv6, IPv4]", func() {
			network.Spec.IPFamilies = []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv6}
			newNetwork := prepareNetworkForUpdate(network)
			newNetwork.Spec.IPFamilies = []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv6, extensionsv1alpha1.IPFamilyIPv4}

			errorList := ValidateNetworkUpdate(newNetwork, network)

			Expect(errorList).To(BeEmpty())
		})

		It("should not allow updating ipFamilies from dual-stack [IPv4, IPv6] to [IPv6, IPv4]", func() {
			network.Spec.IPFamilies = []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4, extensionsv1alpha1.IPFamilyIPv6}
			newNetwork := prepareNetworkForUpdate(network)
			newNetwork.Spec.IPFamilies = []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv6, extensionsv1alpha1.IPFamilyIPv4}

			errorList := ValidateNetworkUpdate(newNetwork, network)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.ipFamilies"),
				})),
			))
		})

		It("should not allow updating ipFamilies from dual-stack [IPv6, IPv4] to [IPv4, IPv6]", func() {
			network.Spec.IPFamilies = []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv6, extensionsv1alpha1.IPFamilyIPv4}
			newNetwork := prepareNetworkForUpdate(network)
			newNetwork.Spec.IPFamilies = []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4, extensionsv1alpha1.IPFamilyIPv6}

			errorList := ValidateNetworkUpdate(newNetwork, network)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.ipFamilies"),
				})),
			))
		})

		It("should prevent updating ipFamilies to an unsupported transition", func() {
			network.Spec.IPFamilies = []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4}
			newNetwork := prepareNetworkForUpdate(network)
			newNetwork.Spec.IPFamilies = []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv6}

			errorList := ValidateNetworkUpdate(newNetwork, network)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.ipFamilies"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.podCIDR"),
					"Detail": Equal("must be a valid IPv6 address"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.serviceCIDR"),
					"Detail": Equal("must be a valid IPv6 address"),
				})),
			))
		})

		It("should not allow removing an address family", func() {
			network.Spec.IPFamilies = []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4, extensionsv1alpha1.IPFamilyIPv6}
			newNetwork := prepareNetworkForUpdate(network)
			newNetwork.Spec.IPFamilies = []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4}

			errorList := ValidateNetworkUpdate(newNetwork, network)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.ipFamilies"),
				})),
			))
		})
	})

	Describe("#ValidateIPFamilies", func() {
		var fldPath *field.Path

		BeforeEach(func() {
			fldPath = field.NewPath("ipFamilies")
		})

		It("should deny unsupported IP families", func() {
			errorList := ValidateIPFamilies([]extensionsv1alpha1.IPFamily{"foo", "bar"}, fldPath)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeNotSupported),
					"Field":    Equal(fldPath.Index(0).String()),
					"BadValue": BeEquivalentTo("foo"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeNotSupported),
					"Field":    Equal(fldPath.Index(1).String()),
					"BadValue": BeEquivalentTo("bar"),
				})),
			))
		})

		It("should deny duplicate IP families", func() {
			errorList := ValidateIPFamilies([]extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4, extensionsv1alpha1.IPFamilyIPv6, extensionsv1alpha1.IPFamilyIPv4, extensionsv1alpha1.IPFamilyIPv6}, fldPath)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeDuplicate),
					"Field":    Equal(fldPath.Index(2).String()),
					"BadValue": Equal(extensionsv1alpha1.IPFamilyIPv4),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeDuplicate),
					"Field":    Equal(fldPath.Index(3).String()),
					"BadValue": Equal(extensionsv1alpha1.IPFamilyIPv6),
				})),
			))
		})

		It("should allow dual-stack IP families", func() {
			ipFamilies := []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4, extensionsv1alpha1.IPFamilyIPv6}
			errorList := ValidateIPFamilies(ipFamilies, fldPath)
			Expect(errorList).To(BeEmpty())
		})

		It("should allow IPv4 single-stack", func() {
			errorList := ValidateIPFamilies([]extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4}, fldPath)
			Expect(errorList).To(BeEmpty())
		})

		It("should allow IPv6 single-stack", func() {
			errorList := ValidateIPFamilies([]extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv6}, fldPath)
			Expect(errorList).To(BeEmpty())
		})
	})
})

func prepareNetworkForUpdate(obj *extensionsv1alpha1.Network) *extensionsv1alpha1.Network {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
