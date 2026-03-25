// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package loadbalancer

import (
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
)

func loadBalancerStatusWithIPs(service *corev1.Service, externalIPs ipSet) *corev1.LoadBalancerStatus {
	ingresses := make([]corev1.LoadBalancerIngress, 0, externalIPs.Len())

	if slices.Contains(service.Spec.IPFamilies, corev1.IPv4Protocol) {
		ingresses = append(ingresses, corev1.LoadBalancerIngress{
			IP: externalIPs.ipv4.String(),

			// TODO: Technically, ipMode=Proxy is correct, but it breaks communication from within the cluster to the load
			//  balancer IP. I.e., we can't get this to work without the kube-proxy "shortcut" that is disabled by ipMode=Proxy
			// IPMode: ptr.To(corev1.LoadBalancerIPModeProxy),
		})
	}

	if slices.Contains(service.Spec.IPFamilies, corev1.IPv6Protocol) {
		ingresses = append(ingresses, corev1.LoadBalancerIngress{
			IP: externalIPs.ipv6.String(),
		})
	}

	return &corev1.LoadBalancerStatus{
		Ingress: ingresses,
	}
}

// debugPortName returns a human-readable name for the given service port, either the port's name or <number>/<protocol>
// if the name is not set.
func debugPortName(port corev1.ServicePort) string {
	if port.Name != "" {
		return port.Name
	}

	return fmt.Sprintf("%d/%s", port.Port, port.Protocol)
}
