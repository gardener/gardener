// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	corednsconstants "github.com/gardener/gardener/pkg/component/networking/coredns/constants"
	nodelocaldnsconstants "github.com/gardener/gardener/pkg/component/networking/nodelocaldns/constants"
	"github.com/gardener/gardener/pkg/provider-local/cloud-provider/loadbalancer"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

var (
	machineSelector = map[string]string{"app": "machine"}
	bastionSelector = map[string]string{"app": "bastion"}
)

func reconcileNetworkPolicies(ctx context.Context, cl client.Client, namespace string, cluster *extensionscontroller.Cluster) error {
	for _, obj := range networkPolicies(namespace, cluster) {
		if err := cl.Patch(ctx, obj, client.Apply, local.FieldOwner, client.ForceOwnership); err != nil {
			return err
		}
	}
	return nil
}

func networkPolicies(namespace string, cluster *extensionscontroller.Cluster) []client.Object {
	denyAll := emptyNetworkPolicy("provider-local-deny-all", namespace)
	denyAll.Spec = networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
	}

	allowMachinePods := emptyNetworkPolicy("provider-local-allow-machine-pods", namespace)
	allowMachinePods.Spec = networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{
			MatchLabels: machineSelector,
		},
		Ingress: []networkingv1.NetworkPolicyIngressRule{
			{From: loadBalancerPeers()},
			{From: machinePodPeers()}, // allow intra-machine communication
			{From: bastionPodPeers(), Ports: sshPort()},
		},
		Egress: []networkingv1.NetworkPolicyEgressRule{
			allowToIstioGateways(cluster), // kube-proxy might short-circuit traffic to the istio-ingressgateway pods
			allowToKindNetwork(),          // required to reach registry
			allowToNodeLocalDNS(),         // machine pods explicitly use node-local DNS.
			{To: loadBalancerPeers()},
			{To: machinePodPeers()},
		},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
	}

	allowBastionPods := emptyNetworkPolicy("provider-local-allow-bastion-pods", namespace)
	allowBastionPods.Spec = networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{
			MatchLabels: bastionSelector,
		},
		Ingress: []networkingv1.NetworkPolicyIngressRule{
			{From: loadBalancerPeers()},
		},
		Egress: []networkingv1.NetworkPolicyEgressRule{
			allowToDNS(),         // bastion pods use the in-cluster CoreDNS to resolve hostnames of machine pods.
			allowToKindNetwork(), // required to reach registry
			{To: machinePodPeers(), Ports: sshPort()},
		},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
	}

	return []client.Object{allowMachinePods, allowBastionPods, denyAll}
}

func machinePodPeers() []networkingv1.NetworkPolicyPeer {
	return []networkingv1.NetworkPolicyPeer{{
		PodSelector: &metav1.LabelSelector{MatchLabels: machineSelector},
	}}
}

func bastionPodPeers() []networkingv1.NetworkPolicyPeer {
	return []networkingv1.NetworkPolicyPeer{{
		PodSelector: &metav1.LabelSelector{MatchLabels: bastionSelector},
	}}
}

func loadBalancerPeers() []networkingv1.NetworkPolicyPeer {
	return []networkingv1.NetworkPolicyPeer{
		{IPBlock: &networkingv1.IPBlock{CIDR: loadbalancer.InternalRangeV4}},
		{IPBlock: &networkingv1.IPBlock{CIDR: loadbalancer.InternalRangeV6}},
	}
}

func sshPort() []networkingv1.NetworkPolicyPort {
	return []networkingv1.NetworkPolicyPort{
		{Port: new(intstr.FromInt32(22)), Protocol: new(corev1.ProtocolTCP)},
	}
}

func allowToKindNetwork() networkingv1.NetworkPolicyEgressRule {
	return networkingv1.NetworkPolicyEgressRule{
		To: []networkingv1.NetworkPolicyPeer{
			{IPBlock: &networkingv1.IPBlock{CIDR: "172.18.0.0/16"}},
			{IPBlock: &networkingv1.IPBlock{CIDR: "fd00:ff::/64"}},
		},
	}
}

func allowToIstioGateways(cluster *extensionscontroller.Cluster) networkingv1.NetworkPolicyEgressRule {
	istioGatewaysRule := networkingv1.NetworkPolicyEgressRule{
		To: []networkingv1.NetworkPolicyPeer{{
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress}},
			PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
				"app":   "istio-ingressgateway",
				"istio": "ingressgateway",
			}},
		}},
		Ports: []networkingv1.NetworkPolicyPort{
			// TODO(jamand): Drop 8132 once the RemoveHTTPProxyLegacyPort feature gate is removed.
			// The local Extension does not register feature gates, so checking it would panic,
			// adding the NetPol unconditionally does not break anything.
			{Port: new(intstr.FromInt32(8132)), Protocol: new(corev1.ProtocolTCP)},
			{Port: new(intstr.FromInt32(8443)), Protocol: new(corev1.ProtocolTCP)},
			{Port: new(intstr.FromInt32(9443)), Protocol: new(corev1.ProtocolTCP)},
		},
	}
	// For multi-zone seeds, also allow egress to the per-zone istio-ingress namespaces.
	if len(cluster.Seed.Spec.Provider.Zones) > 1 {
		for _, zone := range cluster.Seed.Spec.Provider.Zones {
			istioGatewaysRule.To = append(istioGatewaysRule.To, networkingv1.NetworkPolicyPeer{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress}},
				PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app":   "istio-ingressgateway",
					"istio": "ingressgateway--zone--" + zone,
				}},
			})
		}
	}
	return istioGatewaysRule
}

func allowToNodeLocalDNS() networkingv1.NetworkPolicyEgressRule {
	return networkingv1.NetworkPolicyEgressRule{
		To: []networkingv1.NetworkPolicyPeer{
			{IPBlock: &networkingv1.IPBlock{CIDR: "172.18.255.53/32"}},
			{IPBlock: &networkingv1.IPBlock{CIDR: "fd00:ff::53/128"}},
		},
		Ports: []networkingv1.NetworkPolicyPort{
			{Protocol: new(corev1.ProtocolUDP), Port: new(intstr.FromInt32(53))},
			{Protocol: new(corev1.ProtocolTCP), Port: new(intstr.FromInt32(53))},
		},
	}
}

func allowToDNS() networkingv1.NetworkPolicyEgressRule {
	return networkingv1.NetworkPolicyEgressRule{
		To: []networkingv1.NetworkPolicyPeer{
			{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						corev1.LabelMetadataName: metav1.NamespaceSystem,
					},
				},
				PodSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      corednsconstants.LabelKey,
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{corednsconstants.LabelValue},
					}},
				},
			},
			{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						corev1.LabelMetadataName: metav1.NamespaceSystem,
					},
				},
				PodSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      corednsconstants.LabelKey,
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{nodelocaldnsconstants.LabelValue},
					}},
				},
			},
			// required for node local dns feature, allows egress traffic to node local dns cache
			{
				IPBlock: &networkingv1.IPBlock{
					// node local dns feature is only supported for shoots with IPv4 or IPv6 single-stack networking
					CIDR: fmt.Sprintf("%s/32", nodelocaldnsconstants.IPVSAddress),
				},
			},
			{
				IPBlock: &networkingv1.IPBlock{
					// node local dns feature is only supported for shoots with IPv4 or IPv6 single-stack networking
					CIDR: fmt.Sprintf("%s/128", nodelocaldnsconstants.IPVSIPv6Address),
				},
			},
		},
		Ports: []networkingv1.NetworkPolicyPort{
			{Protocol: new(corev1.ProtocolUDP), Port: new(intstr.FromInt32(corednsconstants.PortServiceServer))},
			{Protocol: new(corev1.ProtocolTCP), Port: new(intstr.FromInt32(corednsconstants.PortServiceServer))},
			{Protocol: new(corev1.ProtocolUDP), Port: new(intstr.FromInt32(corednsconstants.PortServer))},
			{Protocol: new(corev1.ProtocolTCP), Port: new(intstr.FromInt32(corednsconstants.PortServer))},
		},
	}
}

func emptyNetworkPolicy(name, namespace string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: networkingv1.SchemeGroupVersion.String(),
			Kind:       "NetworkPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "provider-local",
			},
		},
	}
}
