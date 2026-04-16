// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package loadbalancer

import (
	"fmt"
	"net/netip"

	corev1 "k8s.io/api/core/v1"
)

func getInternalNodeIPs(node *corev1.Node) (ipPair, error) {
	var out ipPair

	for _, addr := range node.Status.Addresses {
		if addr.Type != corev1.NodeInternalIP {
			continue
		}

		ip, err := netip.ParseAddr(addr.Address)
		if err != nil {
			return out, fmt.Errorf("could not parse internal node IP %q: %w", addr.Address, err)
		}

		if ip.Is4() {
			if out.ipv4.IsValid() && out.ipv4 != ip {
				return out, fmt.Errorf("multiple internal IPv4 addresses found for node: %q and %q", out.ipv4, ip)
			}
			out.ipv4 = ip
		} else if ip.Is6() {
			if out.ipv6.IsValid() && out.ipv6 != ip {
				return out, fmt.Errorf("multiple internal IPv6 addresses found for node: %q and %q", out.ipv6, ip)
			}
			out.ipv6 = ip
		}
	}

	if out.Len() == 0 {
		return out, fmt.Errorf("no address of type %s found", corev1.NodeInternalIP)
	}

	return out, nil
}

func getHostIPs(pod *corev1.Pod) (ipPair, error) {
	var out ipPair

	for _, addr := range pod.Status.HostIPs {
		ip, err := netip.ParseAddr(addr.IP)
		if err != nil {
			return out, fmt.Errorf("could not parse host IP %q: %w", addr, err)
		}

		if ip.Is4() {
			if out.ipv4.IsValid() {
				return out, fmt.Errorf("multiple host IPv4 addresses found for pod: %q and %q", out.ipv4, ip)
			}
			out.ipv4 = ip
		} else if ip.Is6() {
			if out.ipv6.IsValid() {
				return out, fmt.Errorf("multiple host IPv6 addresses found for pod: %q and %q", out.ipv6, ip)
			}
			out.ipv6 = ip
		}
	}

	return out, nil
}

func ipStringOrZero(ip netip.Addr) string {
	if !ip.IsValid() {
		return ""
	}
	return ip.String()
}

// debugPortName returns a human-readable name for the given service port, either the port's name or <number>/<protocol>
// if the name is not set.
func debugPortName(port corev1.ServicePort) string {
	if port.Name != "" {
		return port.Name
	}

	return fmt.Sprintf("%d/%s", port.Port, port.Protocol)
}
