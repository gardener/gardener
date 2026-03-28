// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package loadbalancer

import (
	"fmt"
	"maps"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	corev1 "k8s.io/api/core/v1"
)

func getLoadBalancerStatusFromContainer(service *corev1.Service, networkSettings *container.NetworkSettings) (*corev1.LoadBalancerStatus, error) {
	ips, err := getLoadBalancerIPsFromContainer(networkSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to get external IPs for container: %w", err)
	}

	return loadBalancerStatusWithIPs(service, ips.external), nil
}

func getLoadBalancerIPsFromContainer(networkSettings *container.NetworkSettings) (*loadBalancerIPs, error) {
	if networkSettings == nil {
		return nil, fmt.Errorf("missing network settings")
	}

	containerIPs, ok := networkSettings.Networks[defaultNetwork]
	if !ok {
		return nil, fmt.Errorf("container is missing network settings for network %s", defaultNetwork)
	}

	ips := new(loadBalancerIPs)

	if containerIPs.IPAddress != "" {
		parsedIPv4, err := netip.ParseAddr(containerIPs.IPAddress)
		if err != nil {
			return nil, fmt.Errorf("could not parse internal IPv4 address %q: %w", containerIPs.IPAddress, err)
		}
		ips.internal.ipv4 = parsedIPv4
	}
	if containerIPs.GlobalIPv6Address != "" {
		parsedIPv6, err := netip.ParseAddr(containerIPs.GlobalIPv6Address)
		if err != nil {
			return nil, fmt.Errorf("could not parse internal IPv6 address %q: %w", containerIPs.GlobalIPv6Address, err)
		}
		ips.internal.ipv6 = parsedIPv6
	}

	for _, bindings := range networkSettings.Ports {
		for _, binding := range bindings {
			parsedIP, err := netip.ParseAddr(binding.HostIP)
			if err != nil {
				return nil, fmt.Errorf("could not parse IP %q: %w", binding.HostIP, err)
			}

			if parsedIP.Is4() {
				if ips.external.ipv4.IsValid() && ips.external.ipv4 != parsedIP {
					return nil, fmt.Errorf("container has multiple external IPv4 addresses: %s and %s", ips.external.ipv4, parsedIP)
				}
				ips.external.ipv4 = parsedIP
			} else {
				if ips.external.ipv6.IsValid() && ips.external.ipv6 != parsedIP {
					return nil, fmt.Errorf("container has multiple external IPv6 addresses: %s and %s", ips.external.ipv6, parsedIP)
				}
				ips.external.ipv6 = parsedIP
			}
		}
	}

	return ips, nil
}

func portBindingsForService(service *corev1.Service, externalIPs []netip.Addr) (nat.PortMap, error) {
	portMap := nat.PortMap{}
	for _, port := range service.Spec.Ports {
		supportedProtocols := []corev1.Protocol{corev1.ProtocolTCP, corev1.ProtocolUDP}
		if !slices.Contains(supportedProtocols, port.Protocol) {
			return nil, fmt.Errorf("unsupported protocol %s for port %s, expected on of: %v", port.Protocol, debugPortName(port), supportedProtocols)
		}

		portBindings := make([]nat.PortBinding, len(externalIPs))
		for i, ip := range externalIPs {
			portBindings[i] = nat.PortBinding{
				HostIP:   ip.String(),
				HostPort: strconv.FormatInt(int64(port.Port), 10),
			}
		}

		containerPort := nat.Port(fmt.Sprintf("%d/%s", port.Port, strings.ToLower(string(port.Protocol))))
		portMap[containerPort] = portBindings
	}

	return portMap, nil
}

// containerHasDesiredPortBindings checks if the given host config has the port bindings that correspond to the service
// ports without checking the external IPs. I.e., it calculates the list of desired container ports for the service
// and checks if the host config has bindings for those ports.
func containerHasDesiredPortBindings(service *corev1.Service, networkSettings *container.NetworkSettings) (bool, error) {
	desiredPortBindings, err := portBindingsForService(service, nil)
	if err != nil {
		return false, err
	}

	desiredPorts := slices.Sorted(maps.Keys(desiredPortBindings))
	actualPorts := slices.Sorted(maps.Keys(networkSettings.Ports))

	return slices.Equal(desiredPorts, actualPorts), nil
}
