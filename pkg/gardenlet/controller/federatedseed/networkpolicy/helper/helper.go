// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

const AllowToSeedAPIServer = "allow-to-seed-apiserver"

// GetEgressRules creates Network Policy egress rules from a given endpoint resource
func GetEgressRules(kubernetesEndpoints *corev1.Endpoints) []networkingv1.NetworkPolicyEgressRule {
	var egressRules []networkingv1.NetworkPolicyEgressRule

	for _, subset := range kubernetesEndpoints.Subsets {
		egressRule := networkingv1.NetworkPolicyEgressRule{}
		for _, address := range subset.Addresses {
			egressRule.To = append(egressRule.To, networkingv1.NetworkPolicyPeer{
				IPBlock: &networkingv1.IPBlock{
					CIDR: fmt.Sprintf("%s/32", address.IP),
				},
			})
		}

		for _, port := range subset.Ports {
			// do not use named port as this is e.g not support on Weave
			parse := intstr.FromInt(int(port.Port))
			networkPolicyPort := networkingv1.NetworkPolicyPort{
				Port:     &parse,
				Protocol: &port.Protocol,
			}
			egressRule.Ports = append(egressRule.Ports, networkPolicyPort)
		}
		egressRules = append(egressRules, egressRule)
	}
	return egressRules
}

// CreateOrUpdateNetworkPolicy creates or updates the Network Policy 'allow-to-seed-apiserver' in the given namespace
func CreateOrUpdateNetworkPolicy(ctx context.Context, seedClient client.Client, namespace string, egressRules []networkingv1.NetworkPolicyEgressRule) error {
	policy := networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AllowToSeedAPIServer,
			Namespace: namespace,
		},
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, seedClient, &policy, func() error {
		MutatePolicy(&policy, egressRules)
		return nil
	}); err != nil {
		return fmt.Errorf("failed to create / update NetworkPolicy %q in namespace %q: %v", AllowToSeedAPIServer, namespace, err)
	}
	return nil
}

// MutatePolicy mutates a given network policy with given egress rules
func MutatePolicy(policy *networkingv1.NetworkPolicy, egressRules []networkingv1.NetworkPolicyEgressRule) {
	policy.Annotations = map[string]string{
		"gardener.cloud/description": "Allows Egress from pods labeled with 'networking.gardener.cloud/to-seed-apiserver=allowed' to Seed's Kubernetes API Server endpoints in the default namespace.",
	}

	policy.Spec = networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyToSeedAPIServer: v1beta1constants.LabelNetworkPolicyAllowed},
		},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		Egress:      egressRules,
		Ingress:     []networkingv1.NetworkPolicyIngressRule{},
	}
}
