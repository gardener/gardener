// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/pkg/api/extensions/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var _ = Describe("Helper", func() {
	var (
		ipv4 = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4}
		ipv6 = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6}
	)

	DescribeTable("#GetDNSRecordType",
		func(address string, expected extensionsv1alpha1.DNSRecordType) {
			Expect(GetDNSRecordType(address)).To(Equal(expected))
		},

		Entry("valid IPv4 address", "1.2.3.4", extensionsv1alpha1.DNSRecordTypeA),
		Entry("valid IPv6 address", "2001:db8:f00::1", extensionsv1alpha1.DNSRecordTypeAAAA),
		Entry("anything else", "foo", extensionsv1alpha1.DNSRecordTypeCNAME),
	)

	DescribeTable("#GetDNSRecordTTL",
		func(ttl *int64, expected int64) {
			Expect(GetDNSRecordTTL(ttl)).To(Equal(expected))
		},

		Entry("nil value", nil, int64(120)),
		Entry("non-nil value", new(int64(300)), int64(300)),
	)

	Describe("#DNSValuesFromIngress", func() {
		It("should prefer IPs over hostnames", func() {
			ingress := []corev1.LoadBalancerIngress{{IP: "1.2.3.4"}, {Hostname: "lb.example.com"}}

			values, recordType, err := DNSValuesFromIngress(ingress, ipv4)

			Expect(err).NotTo(HaveOccurred())
			Expect(recordType).To(Equal(extensionsv1alpha1.DNSRecordTypeA))
			Expect(values).To(ConsistOf("1.2.3.4"))
		})

		It("should fall back to a single hostname (CNAME) when no IPs are present", func() {
			ingress := []corev1.LoadBalancerIngress{{Hostname: "lb-1.example.com"}, {Hostname: "lb-2.example.com"}}

			values, recordType, err := DNSValuesFromIngress(ingress, ipv4)

			Expect(err).NotTo(HaveOccurred())
			Expect(recordType).To(Equal(extensionsv1alpha1.DNSRecordTypeCNAME))
			// CNAME records must have a single value.
			Expect(values).To(Equal([]string{"lb-1.example.com"}))
		})

		It("should select the IPs of the first configured IP family with a match", func() {
			ingress := []corev1.LoadBalancerIngress{{IP: "1.2.3.4"}, {IP: "fd12::1"}}

			values, recordType, err := DNSValuesFromIngress(ingress, []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6, gardencorev1beta1.IPFamilyIPv4})

			Expect(err).NotTo(HaveOccurred())
			Expect(recordType).To(Equal(extensionsv1alpha1.DNSRecordTypeAAAA))
			Expect(values).To(ConsistOf("fd12::1"))
		})

		It("should error on empty ingress with a distinct message", func() {
			_, _, err := DNSValuesFromIngress(nil, ipv4)
			Expect(err).To(MatchError("exposure has no ingress yet"))
		})

		It("should error when ingress entries have neither IP nor hostname", func() {
			_, _, err := DNSValuesFromIngress([]corev1.LoadBalancerIngress{{}}, ipv4)
			Expect(err).To(MatchError(ContainSubstring("neither IPs nor hostnames")))
		})

		It("should error when IPs match no configured IP family, even if hostnames are present", func() {
			_, _, err := DNSValuesFromIngress([]corev1.LoadBalancerIngress{{IP: "1.2.3.4", Hostname: "lb.example.com"}}, ipv6)
			Expect(err).To(MatchError(ContainSubstring("configured IP family")))
		})
	})

	Describe("#DNSValuesFromNodes", func() {
		node := func(name string, addresses ...corev1.NodeAddress) corev1.Node {
			return corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: name},
				Status:     corev1.NodeStatus{Addresses: addresses},
			}
		}
		externalIP := func(address string) corev1.NodeAddress {
			return corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: address}
		}
		internalIP := func(address string) corev1.NodeAddress {
			return corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: address}
		}

		It("should pick each node's first address in the given type order (internal-first)", func() {
			nodes := []corev1.Node{
				node("cp-1", externalIP("1.2.3.4"), internalIP("10.0.0.1")),
				node("cp-2", externalIP("1.2.3.5")),
			}

			values, recordType, err := DNSValuesFromNodes(nodes, ipv4, corev1.NodeInternalIP, corev1.NodeExternalIP)

			Expect(err).NotTo(HaveOccurred())
			Expect(recordType).To(Equal(extensionsv1alpha1.DNSRecordTypeA))
			Expect(values).To(Equal([]string{"10.0.0.1", "1.2.3.5"}))
		})

		It("should pick each node's first address in the given type order (external-first)", func() {
			nodes := []corev1.Node{
				node("cp-1", externalIP("1.2.3.4"), internalIP("10.0.0.1")),
				node("cp-2", internalIP("10.0.0.2")),
			}

			values, recordType, err := DNSValuesFromNodes(nodes, ipv4, corev1.NodeExternalIP, corev1.NodeInternalIP)

			Expect(err).NotTo(HaveOccurred())
			Expect(recordType).To(Equal(extensionsv1alpha1.DNSRecordTypeA))
			Expect(values).To(Equal([]string{"1.2.3.4", "10.0.0.2"}))
		})

		It("should select the first IP family with at least one matching address", func() {
			nodes := []corev1.Node{
				node("cp-1", internalIP("10.0.0.1")),
				node("cp-2", internalIP("fd12::1")),
			}

			values, recordType, err := DNSValuesFromNodes(nodes, []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4, gardencorev1beta1.IPFamilyIPv6}, corev1.NodeInternalIP)

			Expect(err).NotTo(HaveOccurred())
			Expect(recordType).To(Equal(extensionsv1alpha1.DNSRecordTypeA))
			Expect(values).To(Equal([]string{"10.0.0.1"}))
		})

		It("should ignore addresses that are no IP of the family, e.g. hostnames", func() {
			nodes := []corev1.Node{
				node("cp-1", corev1.NodeAddress{Type: corev1.NodeInternalDNS, Address: "internal.dns"}, externalIP("1.2.3.4")),
			}

			values, recordType, err := DNSValuesFromNodes(nodes, ipv4, corev1.NodeInternalDNS, corev1.NodeExternalIP)

			Expect(err).NotTo(HaveOccurred())
			Expect(recordType).To(Equal(extensionsv1alpha1.DNSRecordTypeA))
			Expect(values).To(Equal([]string{"1.2.3.4"}))
		})

		It("should error if a node has no addresses", func() {
			_, _, err := DNSValuesFromNodes([]corev1.Node{node("cp-1")}, ipv4, corev1.NodeInternalIP)
			Expect(err).To(MatchError(ContainSubstring(`node "cp-1" has no addresses`)))
		})

		It("should error if no address of the given types matches a configured IP family", func() {
			_, _, err := DNSValuesFromNodes([]corev1.Node{node("cp-1", internalIP("10.0.0.1"))}, ipv6, corev1.NodeInternalIP, corev1.NodeExternalIP)
			Expect(err).To(MatchError(ContainSubstring("configured IP family")))
		})

		It("should error if no node address is of the given types", func() {
			_, _, err := DNSValuesFromNodes([]corev1.Node{node("cp-1", internalIP("10.0.0.1"))}, ipv4, corev1.NodeExternalIP)
			Expect(err).To(MatchError(ContainSubstring("configured IP family")))
		})
	})
})
