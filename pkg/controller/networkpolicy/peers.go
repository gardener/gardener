// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy

import (
	"net"

	networkingv1 "k8s.io/api/networking/v1"
)

// allPrivateNetworkBlocksV4 returns a list of all Private network (RFC1918) and Carrier-grade NAT (RFC6598) IPv4 blocks.
func allPrivateNetworkBlocksV4() []net.IPNet {
	return []net.IPNet{
		// 10.0.0.0/8 (private network (RFC1918))
		{IP: net.IP{10, 0, 0, 0}, Mask: net.CIDRMask(8, 32)},
		// 172.16.0.0/12 (private network (RFC1918))
		{IP: net.IP{172, 16, 0, 0}, Mask: net.CIDRMask(12, 32)},
		// 192.168.0.0/16 (private network (RFC1918))
		{IP: net.IP{192, 168, 0, 0}, Mask: net.CIDRMask(16, 32)},
		// 100.64.0.0/10 (Carrier-grade NAT (RFC6598))
		{IP: net.IP{100, 64, 0, 0}, Mask: net.CIDRMask(10, 32)},
	}
}

// allPrivateNetworkBlocksV6 returns a list of all private reserved IPv6 network blocks.
// See https://en.wikipedia.org/wiki/Reserved_IP_addresses#IPv6.
func allPrivateNetworkBlocksV6() []net.IPNet {
	return []net.IPNet{
		// fe80::/10 (Link Local)
		{IP: net.IP{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, Mask: net.CIDRMask(10, 128)},
		// fc00::/7 (Unique Local (ULA))
		{IP: net.IP{0xfc, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, Mask: net.CIDRMask(7, 128)},
	}
}

// toCIDRStrings takes a list of net.IPNet and returns their CIDR representations in a string slice.
func toCIDRStrings(networks ...net.IPNet) []string {
	var out []string
	for _, network := range networks {
		out = append(out, network.String())
	}
	return out
}

// toNetworkPolicyPeersWithExceptions returns a list of networkingv1.NetworkPolicyPeers whose ipBlock.cidr points to
// `networks` and whose ipBlock.except points to `except`.
func toNetworkPolicyPeersWithExceptions(networks []net.IPNet, except ...string) ([]networkingv1.NetworkPolicyPeer, error) {
	var result []networkingv1.NetworkPolicyPeer

	for _, n := range networks {
		excluded, err := excludeBlock(&n, except...)
		if err != nil {
			return nil, err
		}

		result = append(result, networkingv1.NetworkPolicyPeer{
			IPBlock: &networkingv1.IPBlock{
				CIDR:   n.String(),
				Except: excluded,
			},
		})
	}

	return result, nil
}

// networkPolicyPeersWithExceptions returns a list of networkingv1.NetworkPolicyPeers whose ipBlock.cidr points to
// `networks` and whose ipBlock.except points to `except`.
func networkPolicyPeersWithExceptions(networks []string, except ...string) ([]networkingv1.NetworkPolicyPeer, error) {
	var ipNets []net.IPNet

	for _, n := range networks {
		_, net, err := net.ParseCIDR(n)
		if err != nil {
			return nil, err
		}

		ipNets = append(ipNets, *net)
	}

	return toNetworkPolicyPeersWithExceptions(ipNets, except...)
}

func excludeBlock(parentBlock *net.IPNet, cidrs ...string) ([]string, error) {
	var matchedCIDRs []string

	for _, cidr := range cidrs {
		ip, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}
		parentBlockMaskLength, _ := parentBlock.Mask.Size()
		ipNetMaskLength, _ := ipNet.Mask.Size()

		if parentBlock.Contains(ip) && parentBlockMaskLength < ipNetMaskLength {
			matchedCIDRs = append(matchedCIDRs, cidr)
		}
	}

	return matchedCIDRs, nil
}
