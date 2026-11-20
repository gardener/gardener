// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy

import (
	"net/netip"
	"slices"

	networkingv1 "k8s.io/api/networking/v1"
)

// PrivateNetworks returns a list of all private reserved network blocks.
func PrivateNetworks() []string {
	return []string{
		// IPv4
		"10.0.0.0/8",     // (private network (RFC1918))
		"100.64.0.0/10",  // (Carrier-grade NAT (RFC6598))
		"172.16.0.0/12",  // (private network (RFC1918))
		"192.168.0.0/16", // (private network (RFC1918))
		// IPv6
		// See https://en.wikipedia.org/wiki/Reserved_IP_addresses#IPv6.
		"fc00::/7",  // (Unique Local (ULA))
		"fe80::/10", // (Link Local)
	}
}

// AllNetworks returns a list of all IPv4 and IPv6 networks CIDRs.
func AllNetworks() []string {
	return []string{
		"0.0.0.0/0",
		"::/0",
	}
}

// GetNetworkPolicyPeersWithExceptions returns a NetworkPolicyPeer slice which allows traffic to all peers while adding all applicable exceptions to each peer.
// If a whole peer would be excluded by an exception, this exception is NOT applied.
func GetNetworkPolicyPeersWithExceptions(peers []string, exceptions []string) ([]networkingv1.NetworkPolicyPeer, error) {
	var result []networkingv1.NetworkPolicyPeer

	peerPrefixes, err := parseCIDRStringsToNormalizedPrefixes(peers)
	if err != nil {
		return nil, err
	}
	exceptionPrefixes, err := parseCIDRStringsToNormalizedPrefixes(exceptions)
	if err != nil {
		return nil, err
	}

	peerPrefixes = getDistinctNetworks(peerPrefixes)
	exceptionPrefixes = getDistinctNetworks(exceptionPrefixes)

	for _, peer := range peerPrefixes {
		exceptionsForPeer := getExceptionsForPeer(peer, exceptionPrefixes)

		result = append(result, networkingv1.NetworkPolicyPeer{
			IPBlock: &networkingv1.IPBlock{
				CIDR:   peer.String(),
				Except: exceptionsForPeer,
			},
		})
	}
	return result, nil
}

func parseCIDRStringsToNormalizedPrefixes(cidrs []string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(cidrs))
	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, normalizePrefix(prefix))
	}
	return prefixes, nil
}

// Treat IPv4-mapped IPv6 addresses as IPv4 addresses and normalize the CIDR.
func normalizePrefix(input netip.Prefix) netip.Prefix {
	if !input.Addr().Is4In6() {
		return input.Masked()
	}
	// Create a new Prefix by unmapping the address and subtracting the IPv4-in-IPv6 offset (96=128-32).
	return netip.PrefixFrom(input.Addr().Unmap(), input.Bits()-96).Masked()
}

// getDistinctNetworks returns the minimal set of input networks such that no network is a subset of another.
func getDistinctNetworks(networks []netip.Prefix) []netip.Prefix {
	if len(networks) == 0 {
		return networks
	}

	// Sort by address family, then masked prefix address, then prefix length, then unmasked address.
	slices.SortFunc(networks, netip.Prefix.Compare)

	distinctNetworks := []netip.Prefix{networks[0]}
	for _, net := range networks {
		// Since the networks are sorted, it is enough to check if the network is contained in the previous distinct network.
		if !distinctNetworks[len(distinctNetworks)-1].Contains(net.Addr()) {
			distinctNetworks = append(distinctNetworks, net)
		}
	}
	return distinctNetworks
}

func getExceptionsForPeer(peer netip.Prefix, exceptions []netip.Prefix) []string {
	exceptionsForPeer := []string{}

	for _, exception := range exceptions {
		// Only add exceptions that are within the peer and which have a larger Netmask.
		if peer.Overlaps(exception) && peer.Bits() < exception.Bits() {
			exceptionsForPeer = append(exceptionsForPeer, exception.String())
		}
	}
	return exceptionsForPeer
}
