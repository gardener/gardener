// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	. "github.com/gardener/gardener/pkg/controller/networkpolicy/helper"
)

var _ = Describe("helper", func() {
	Describe("#GetEgressRules", func() {
		It("should return and empty EgressRule", func() {
			Expect(GetEgressRules()).To(BeEmpty())
		})

		It("should return the Egress rule with correct IP Blocks", func() {
			var (
				ip1     = "10.250.119.142"
				ip2     = "10.250.119.143"
				ip3     = "10.250.119.144"
				ip4     = "10.250.119.145"
				subsets = []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: ip1,
							},
							{
								IP: ip2,
							},
							{
								IP: ip2, // duplicate address should be removed
							},
						},
					},
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: ip3,
							},
							{
								IP: ip4,
							},
							{
								IP: ip2, // should not be removed, no duplicate in this EndpointAddress list
							},
							{
								IP: ip4, // duplicate address should be removed
							},
						},
						NotReadyAddresses: []corev1.EndpointAddress{
							{
								IP: "10.250.119.146",
							},
						},
					},
				}
			)

			egressRules, err := GetEgressRules(subsets...)
			expectedRules := []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: ip1 + "/32",
							},
						},
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: ip2 + "/32",
							},
						},
					},
				},
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: ip3 + "/32",
							},
						},
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: ip4 + "/32",
							},
						},
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: ip2 + "/32",
							},
						},
					},
				},
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(egressRules).To(Equal(expectedRules))
		})

		It("should return the Egress rule with correct IP Blocks of 2 same EndpointSubset", func() {
			var (
				ip1     = "10.250.119.142"
				tcp     = corev1.ProtocolTCP
				subsets = []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: ip1,
							},
						},
						Ports: []corev1.EndpointPort{
							{
								Port:     443,
								Protocol: tcp,
							},
						},
					},
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: ip1,
							},
						},
						Ports: []corev1.EndpointPort{
							{
								Port:     443,
								Protocol: tcp,
							},
						},
					},
				}
			)

			egressRules, err := GetEgressRules(subsets...)

			port443 := intstr.FromInt32(443)
			expectedRules := []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: ip1 + "/32",
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &tcp,
							Port:     &port443,
						},
					},
				},
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: ip1 + "/32",
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &tcp,
							Port:     &port443,
						},
					},
				},
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(egressRules).To(Equal(expectedRules))
		})

		It("should return the Egress rule with correct Ports", func() {
			var (
				tcp     = corev1.ProtocolTCP
				udp     = corev1.ProtocolUDP
				subsets = []corev1.EndpointSubset{
					{
						Ports: []corev1.EndpointPort{
							{
								Protocol: tcp,
								Port:     443,
							},
						},
					},
					{
						Ports: []corev1.EndpointPort{
							{
								Protocol: corev1.ProtocolUDP,
								Port:     161,
							},
						},
					},
				}
			)

			egressRules, err := GetEgressRules(subsets...)
			port443 := intstr.FromInt32(443)
			port161 := intstr.FromInt32(161)
			expectedRules := []networkingv1.NetworkPolicyEgressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &tcp,
							Port:     &port443,
						},
					},
				},
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &udp,
							Port:     &port161,
						},
					},
				},
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(egressRules).To(Equal(expectedRules))
		})
	})
})
