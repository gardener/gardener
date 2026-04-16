// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package loadbalancer

import (
	"context"
	"fmt"
	"net/netip"
	"slices"

	"github.com/docker/docker/api/types/network"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	// InternalRangeV4 is the IPv4 CIDR range from which internal load balancer IPs are allocated.
	// It must be a subset of the kind network.
	InternalRangeV4 = "172.18.0.240/28"
	// ExternalRangeV4 is the IPv4 CIDR range from which load balancer IPs are allocated.
	// It must not overlap with the kind network.
	ExternalRangeV4 = "172.18.255.240/28"
	// InternalRangeV6 is the IPv6 CIDR range from which internal load balancer IPs are allocated.
	// It must be a subset of the kind network.
	InternalRangeV6 = "fd00:10::f0/124"
	// ExternalRangeV6 is the IPv6 CIDR range from which load balancer IPs are allocated.
	// It must not overlap with the kind network.
	ExternalRangeV6 = "fd00:ff::f0/124"
)

var (
	internalPrefixV4, externalPrefixV4 netip.Prefix
	internalPrefixV6, externalPrefixV6 netip.Prefix
)

func init() {
	internalPrefixV4 = netip.MustParsePrefix(InternalRangeV4)
	externalPrefixV4 = netip.MustParsePrefix(ExternalRangeV4)
	internalPrefixV6 = netip.MustParsePrefix(InternalRangeV6)
	externalPrefixV6 = netip.MustParsePrefix(ExternalRangeV6)
}

type loadBalancerIPs struct {
	internal, external ipPair
}

type ipPair struct {
	ipv4, ipv6 netip.Addr
}

func (s ipPair) Len() int {
	var l int
	if s.ipv4.IsValid() {
		l++
	}
	if s.ipv6.IsValid() {
		l++
	}
	return l
}

func (s ipPair) AsSlice() []netip.Addr {
	out := make([]netip.Addr, 0, s.Len())
	if s.ipv4.IsValid() {
		out = append(out, s.ipv4)
	}
	if s.ipv6.IsValid() {
		out = append(out, s.ipv6)
	}
	return out
}

func (s ipPair) PreferredAddr(ipFamilies []corev1.IPFamily) netip.Addr {
	if slices.Contains(ipFamilies, corev1.IPv6Protocol) && s.ipv6.IsValid() {
		return s.ipv6
	}
	return s.ipv4
}

func (p *Provider) allocateLoadBalancerIPs(ctx context.Context, service *corev1.Service) (*loadBalancerIPs, error) {
	info, err := p.DockerClient.NetworkInspect(ctx, kindNetwork, network.InspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to inspect network %s: %w", kindNetwork, err)
	}

	allocatedIPs := sets.New[netip.Addr]()
	for _, c := range info.Containers {
		if c.IPv4Address != "" {
			parsedIPv4, err := netip.ParsePrefix(c.IPv4Address)
			if err != nil {
				return nil, fmt.Errorf("could not parse internal IPv4 address %q of container %s: %w", c.IPv4Address, c.Name, err)
			}
			allocatedIPs.Insert(parsedIPv4.Addr())
		}
		if c.IPv6Address != "" {
			parsedIPv6, err := netip.ParsePrefix(c.IPv6Address)
			if err != nil {
				return nil, fmt.Errorf("could not parse internal IPv6 address %q of container %s: %w", c.IPv6Address, c.Name, err)
			}
			allocatedIPs.Insert(parsedIPv6.Addr())
		}
	}

	ips := &loadBalancerIPs{}

	if slices.Contains(service.Spec.IPFamilies, corev1.IPv4Protocol) {
		ips.internal.ipv4, err = nextFree(internalPrefixV4, allocatedIPs)
		if err != nil {
			return ips, fmt.Errorf("could not allocate internal IPv4 address: %w", err)
		}

		ips.external.ipv4 = toExternalIP(ips.internal.ipv4)
	}

	if slices.Contains(service.Spec.IPFamilies, corev1.IPv6Protocol) {
		ips.internal.ipv6, err = nextFree(internalPrefixV6, allocatedIPs)
		if err != nil {
			return ips, fmt.Errorf("could not allocate internal IPv6 address: %w", err)
		}

		ips.external.ipv6 = toExternalIP(ips.internal.ipv6)
	}

	return ips, nil
}

func nextFree(prefix netip.Prefix, allocatedIPs sets.Set[netip.Addr]) (netip.Addr, error) {
	for ip := prefix.Addr(); prefix.Contains(ip); ip = ip.Next() {
		if !allocatedIPs.Has(ip) {
			return ip, nil
		}
	}

	return netip.Addr{}, fmt.Errorf("no free IPs in CIDR %s", prefix)
}

func toExternalIP(internalIP netip.Addr) netip.Addr {
	ip := internalIP.AsSlice()
	lastByte := ip[len(ip)-1]

	// Map 172.18.0.240/28 to 172.18.255.240/28 by keeping the last byte
	if internalIP.Is4() {
		externalIP := externalPrefixV4.Addr().As4()
		externalIP[3] = lastByte
		return netip.AddrFrom4(externalIP)
	}

	// Map fd00:10::f0/124 to fd00:ff::f0/124 by keeping the last byte
	externalIP := externalPrefixV6.Addr().As16()
	externalIP[15] = lastByte
	return netip.AddrFrom16(externalIP)
}
