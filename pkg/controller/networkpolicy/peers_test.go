// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	networkingv1 "k8s.io/api/networking/v1"

	"github.com/gardener/gardener/pkg/controller/networkpolicy"
)

var _ = Describe("peers", func() {
	DescribeTable("#GetNetworkPolicyPeersWithExceptions",
		func(peers, exceptions []string, expectedNetworkPolicyPeers []networkingv1.NetworkPolicyPeer, errorMatcher gomegatypes.GomegaMatcher) {
			result, err := networkpolicy.GetNetworkPolicyPeersWithExceptions(peers, exceptions)
			Expect(err).To(errorMatcher)
			Expect(result).To(Equal(expectedNetworkPolicyPeers))
		},
		Entry("empty exceptions",
			[]string{"10.0.0.0/8", "::ab12:0/112"},
			[]string{},
			[]networkingv1.NetworkPolicyPeer{
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "10.0.0.0/8",
						Except: []string{},
					},
				},
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "::ab12:0/112",
						Except: []string{},
					},
				},
			},
			BeNil(),
		),
		Entry("normal CIDRs",
			[]string{"10.0.0.0/8", "::ab12:0/112"},
			[]string{"10.0.0.0/9", "::ab12:8000/113"},
			[]networkingv1.NetworkPolicyPeer{
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "10.0.0.0/8",
						Except: []string{"10.0.0.0/9"},
					},
				},
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "::ab12:0/112",
						Except: []string{"::ab12:8000/113"},
					},
				},
			},
			BeNil(),
		),
		Entry("not normalized CIDRs",
			[]string{"10.1.2.3/8", "::ab12:0/112"},
			[]string{"10.0.0.0/9", "::ab12:1/113"},
			[]networkingv1.NetworkPolicyPeer{
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "10.0.0.0/8",
						Except: []string{"10.0.0.0/9"},
					},
				},
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "::ab12:0/112",
						Except: []string{"::ab12:0/113"},
					},
				},
			},
			BeNil(),
		),
		Entry("whole peer in excludes",
			[]string{"10.0.0.0/8", "11.0.0.0/16"},
			[]string{"10.0.0.0/8", "11.0.0.0/8"},
			[]networkingv1.NetworkPolicyPeer{
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "10.0.0.0/8",
						Except: []string{},
					},
				},
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "11.0.0.0/16",
						Except: []string{},
					},
				},
			},
			BeNil(),
		),
		Entry("IPv4-mapped IPv6 CIDR",
			[]string{"10.0.0.0/8"},
			[]string{"::ffff:10.0.0.0/105"}, // = 10.0.0.0/9
			[]networkingv1.NetworkPolicyPeer{
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "10.0.0.0/8",
						Except: []string{"10.0.0.0/9"},
					},
				},
			},
			BeNil(),
		),
		Entry("exceptions outside peers",
			[]string{"10.0.0.0/8", "::ab12:0/112"},
			[]string{"11.0.0.0/9", "::ab13:8000/113"},
			[]networkingv1.NetworkPolicyPeer{
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "10.0.0.0/8",
						Except: []string{},
					},
				},
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "::ab12:0/112",
						Except: []string{},
					},
				},
			},
			BeNil(),
		),
		Entry("overlapping exceptions",
			[]string{"10.0.0.0/8", "::ab12:0/112"},
			[]string{"10.255.0.0/16", "10.0.0.0/9", "::ab12:f000/116", "10.0.0.0/9", "::ab12:c000/114", "10.0.0.0/10", "10.128.0.0/9", "::ab12:c000/114", "::ab12:0/113"},
			[]networkingv1.NetworkPolicyPeer{
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR: "10.0.0.0/8",
						Except: []string{
							"10.0.0.0/9",
							"10.128.0.0/9",
						},
					},
				},
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR: "::ab12:0/112",
						Except: []string{
							"::ab12:0/113",
							"::ab12:c000/114",
						},
					},
				},
			},
			BeNil(),
		),
		Entry("allow-to-private-networks",
			networkpolicy.PrivateNetworks(),
			[]string{
				"10.0.0.0/16",
				"10.1.0.0/16",
				"10.2.0.0/16",
				"10.3.0.0/16",
			},
			[]networkingv1.NetworkPolicyPeer{
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR: "10.0.0.0/8",
						Except: []string{
							"10.0.0.0/16",
							"10.1.0.0/16",
							"10.2.0.0/16",
							"10.3.0.0/16",
						},
					},
				},
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "100.64.0.0/10",
						Except: []string{},
					},
				},
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "172.16.0.0/12",
						Except: []string{},
					},
				},
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "192.168.0.0/16",
						Except: []string{},
					},
				},
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "fc00::/7",
						Except: []string{},
					},
				},
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "fe80::/10",
						Except: []string{},
					},
				},
			},
			BeNil(),
		),
		Entry("allow-to-public-networks",
			networkpolicy.AllNetworks(),
			append(networkpolicy.PrivateNetworks(), "10.0.0.0/16", "10.1.0.0/16"),
			[]networkingv1.NetworkPolicyPeer{
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR: "0.0.0.0/0",
						Except: []string{
							"10.0.0.0/8",
							"100.64.0.0/10",
							"172.16.0.0/12",
							"192.168.0.0/16",
						},
					},
				},
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR: "::/0",
						Except: []string{
							"fc00::/7",
							"fe80::/10",
						},
					},
				},
			},
			BeNil(),
		),
		Entry("malformed peers",
			[]string{"10.0.0.0/8", "::ab12:0::/112"},
			[]string{"10.0.0.0/9", "::ab12:8000/113"},
			nil,
			HaveOccurred(),
		),
		Entry("malformed exceptions",
			[]string{"10.0.0.0/8", "::ab12:0/112"},
			[]string{"10.0.0.0/9", "::ab12:8000::/113"},
			nil,
			HaveOccurred(),
		),
	)
})
