// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package networkpolicies_test

import (
	"net"

	. "github.com/gardener/gardener/pkg/operation/botanist/component/networkpolicies"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	networkingv1 "k8s.io/api/networking/v1"
)

var _ = Describe("networkpolicies", func() {
	var (
		_, block8, _            = net.ParseCIDR("10.0.0.0/8")
		_, block12, _           = net.ParseCIDR("172.16.0.0/12")
		_, block16, _           = net.ParseCIDR("192.168.0.0/16")
		_, carrierGradeBlock, _ = net.ParseCIDR("100.64.0.0/10")
	)

	Describe("#Private8BitBlock", func() {
		It("should return the correct CIDR", func() {
			Expect(Private8BitBlock()).To(Equal(block8))
		})
	})

	Describe("#Private12BitBlock", func() {
		It("should return the correct CIDR", func() {
			Expect(Private12BitBlock()).To(Equal(block12))
		})
	})

	Describe("#Private16BitBlock", func() {
		It("should return the correct CIDR", func() {
			Expect(Private16BitBlock()).To(Equal(block16))
		})
	})

	Describe("#CarrierGradeNATBlock", func() {
		It("should return the correct CIDR", func() {
			Expect(CarrierGradeNATBlock()).To(Equal(carrierGradeBlock))
		})
	})

	Describe("#AllPrivateNetworkBlocks", func() {
		It("should contain correct CIDRs", func() {
			Expect(AllPrivateNetworkBlocks()).To(ConsistOf(*block8, *block12, *block16, *carrierGradeBlock))
		})
	})

	Describe("#ToNetworkPolicyPeersWithExceptions", func() {
		It("should return correct result", func() {
			result, err := ToNetworkPolicyPeersWithExceptions(
				AllPrivateNetworkBlocks(),
				"10.10.0.0/24",
				"172.16.1.0/24",
				"192.168.1.0/24",
				"100.64.1.0/24",
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ConsistOf([]networkingv1.NetworkPolicyPeer{
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "10.0.0.0/8",
						Except: []string{"10.10.0.0/24"},
					},
				},
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "172.16.0.0/12",
						Except: []string{"172.16.1.0/24"},
					},
				},
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "192.168.0.0/16",
						Except: []string{"192.168.1.0/24"},
					},
				},
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "100.64.0.0/10",
						Except: []string{"100.64.1.0/24"},
					},
				},
			}))
		})

		It("should not have overlapping excepts", func() {
			result, err := ToNetworkPolicyPeersWithExceptions(
				AllPrivateNetworkBlocks(),
				"192.167.0.0/16",
				"192.168.0.0/16",
				"10.10.0.0/24",
				"10.0.0.0/8",
				"100.64.0.0/10",
				"172.16.0.0/12",
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ConsistOf([]networkingv1.NetworkPolicyPeer{
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "10.0.0.0/8",
						Except: []string{"10.10.0.0/24"},
					},
				},
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR: "172.16.0.0/12",
					},
				},
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR: "192.168.0.0/16",
					},
				},
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR: "100.64.0.0/10",
					},
				},
			}))
		})
	})
})
