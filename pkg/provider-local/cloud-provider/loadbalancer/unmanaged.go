// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package loadbalancer

import (
	"net/netip"
	"strings"

	corev1 "k8s.io/api/core/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// On self-hosted shoots with unmanaged infrastructure, we don't provision dynamic load balancers.
// Instead, we configure the kind runtime cluster with static port mappings from the load balancer IPs (bound to the
// host's loopback interface) to hard-coded node ports. The static node ports are configured by the
// MutatingAdmissionPolicy loadbalancer-services.
// getLoadBalancerStatusForUnmanagedInfra checks if the given service is such a static load balancer and, if so, returns
// a LoadBalancerStatus with the hard-coded IPs.
func (p *Provider) getLoadBalancerStatusForUnmanagedInfra(service *corev1.Service, clusterName string) (*corev1.LoadBalancerStatus, bool) {
	// We're running in a self-hosted shoot with unmanaged infrastructure if the clusterName starts with `shoot-`, but
	// there is no client for the runtime cluster. If this is the kind cluster or if we have a client for the
	// runtime cluster, we provision load balancers dynamically and this function is irrelevant.
	if !strings.HasPrefix(clusterName, v1beta1constants.TechnicalIDPrefix) || p.RuntimeClient != nil {
		return nil, false
	}

	var externalIPs ipSet

	switch {
	case service.Namespace == "virtual-garden-istio-ingress" && service.Name == "istio-ingressgateway":
		externalIPs = ipSet{
			ipv4: netip.MustParseAddr("172.18.255.3"),
			ipv6: netip.MustParseAddr("fd00:ff::3"),
		}
	default:
		return nil, false
	}

	return loadBalancerStatusWithIPs(service, externalIPs, corev1.LoadBalancerIPModeVIP), true
}
