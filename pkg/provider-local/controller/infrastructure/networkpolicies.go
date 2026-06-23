// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/provider-local/cloud-provider/loadbalancer"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

const sshPort = 22

func reconcileNetworkPolicies(ctx context.Context, cl client.Client, namespace string, cluster *extensionscontroller.Cluster) error {
	for _, obj := range networkPolicies(namespace, cluster) {
		if err := cl.Patch(ctx, obj, client.Apply, local.FieldOwner, client.ForceOwnership); err != nil {
			return err
		}
	}
	return nil
}

func networkPolicies(namespace string, cluster *extensionscontroller.Cluster) []client.Object {
	// allow-to-istio-ingress-gateway allows egress from machine pods to the Istio ingress gateway,
	// enabling them to reach their shoot control plane (kube-apiserver).
	allowToIstioIngressGateway := emptyNetworkPolicy("allow-to-istio-ingress-gateway", namespace)
	allowToIstioIngressGateway.Spec = networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{local.LabelNetworkPolicyToIstioIngressGateway: v1beta1constants.LabelNetworkPolicyAllowed},
		},
		Egress: []networkingv1.NetworkPolicyEgressRule{{
			To: []networkingv1.NetworkPolicyPeer{{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": "istio-ingress"}},
				PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app":   "istio-ingressgateway",
					"istio": "ingressgateway",
				}},
			}},
			Ports: []networkingv1.NetworkPolicyPort{
				// TODO(hown3d): Drop 8132 with RemoveHTTPProxyLegacyPort feature gate
				{Port: new(intstr.FromInt32(8132)), Protocol: new(corev1.ProtocolTCP)},
				{Port: new(intstr.FromInt32(8443)), Protocol: new(corev1.ProtocolTCP)},
				{Port: new(intstr.FromInt32(9443)), Protocol: new(corev1.ProtocolTCP)},
			},
		}},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
	}
	// For multi-zone seeds, also allow egress to the per-zone istio-ingress namespaces.
	if len(cluster.Seed.Spec.Provider.Zones) > 1 {
		for _, zone := range cluster.Seed.Spec.Provider.Zones {
			allowToIstioIngressGateway.Spec.Egress[0].To = append(allowToIstioIngressGateway.Spec.Egress[0].To, networkingv1.NetworkPolicyPeer{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": "istio-ingress--" + zone}},
				PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app":   "istio-ingressgateway",
					"istio": "ingressgateway--zone--" + zone,
				}},
			})
		}
	}

	// allow-machine-pods allows:
	// - ingress from load balancer containers (envoy, in the internal LB IP range) so they can forward
	//   traffic to machine pods (e.g., VPN on port 30123).
	// - ingress from bastion pods for SSH access (port 22).
	// - ingress and egress between machine pods for inter-node communication.
	allowMachinePods := emptyNetworkPolicy("allow-machine-pods", namespace)
	allowMachinePods.Spec = networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "machine"},
		},
		Ingress: []networkingv1.NetworkPolicyIngressRule{
			{
				From: []networkingv1.NetworkPolicyPeer{
					// Load balancer containers (envoy) run in the kind network with IPs from InternalRangeV4/V6.
					{IPBlock: &networkingv1.IPBlock{CIDR: loadbalancer.InternalRangeV4}},
					{IPBlock: &networkingv1.IPBlock{CIDR: loadbalancer.InternalRangeV6}},
					// Other machine pods in this namespace.
					{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "machine"}}},
				},
			},
			{
				// Bastion pods need SSH access to machine pods.
				From: []networkingv1.NetworkPolicyPeer{
					{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "bastion"}}},
				},
				Ports: []networkingv1.NetworkPolicyPort{
					{Port: new(intstr.FromInt32(sshPort)), Protocol: new(corev1.ProtocolTCP)},
				},
			},
		},
		Egress: []networkingv1.NetworkPolicyEgressRule{{
			To: []networkingv1.NetworkPolicyPeer{
				{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "machine"}}},
			},
		}},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
	}

	// allow-bastion-pods allows:
	// - ingress from load balancer containers (envoy) so the bastion's LoadBalancer service works.
	// - egress to machine pods on port 22 (SSH).
	allowBastionPods := emptyNetworkPolicy("allow-bastion-pods", namespace)
	allowBastionPods.Spec = networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "bastion"},
		},
		Ingress: []networkingv1.NetworkPolicyIngressRule{{
			From: []networkingv1.NetworkPolicyPeer{
				// Load balancer containers (envoy) run in the kind network with IPs from InternalRangeV4/V6.
				{IPBlock: &networkingv1.IPBlock{CIDR: loadbalancer.InternalRangeV4}},
				{IPBlock: &networkingv1.IPBlock{CIDR: loadbalancer.InternalRangeV6}},
			},
		}},
		Egress: []networkingv1.NetworkPolicyEgressRule{{
			To: []networkingv1.NetworkPolicyPeer{
				{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "machine"}}},
			},
			Ports: []networkingv1.NetworkPolicyPort{
				{Port: new(intstr.FromInt32(sshPort)), Protocol: new(corev1.ProtocolTCP)},
			},
		}},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
	}

	return []client.Object{allowToIstioIngressGateway, allowMachinePods, allowBastionPods}
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
		},
	}
}
