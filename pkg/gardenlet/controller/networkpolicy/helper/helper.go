// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helper

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// AllowToSeedAPIServer is the name of the Network Policy that allows egress to the Seed's Kubernetes API Server
// endpoints in the default namespace.
const AllowToSeedAPIServer = "allow-to-seed-apiserver"

// GetEgressRules creates Network Policy egress rules from endpoint subsets.
func GetEgressRules(subsets ...corev1.EndpointSubset) []networkingv1.NetworkPolicyEgressRule {
	var (
		egressRules = []networkingv1.NetworkPolicyEgressRule{}
		existingIPs = sets.NewString()
	)

	for _, subset := range subsets {
		egressRule := networkingv1.NetworkPolicyEgressRule{}

		for _, address := range subset.Addresses {
			if existingIPs.Has(address.IP) {
				continue
			}

			existingIPs.Insert(address.IP)

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

// EnsureNetworkPolicy ensures the Network Policy 'allow-to-seed-apiserver' in the given namespace
func EnsureNetworkPolicy(ctx context.Context, seedClient client.Client, namespace string, egressRules []networkingv1.NetworkPolicyEgressRule) error {
	policy := networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AllowToSeedAPIServer,
			Namespace: namespace,
		},
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, seedClient, &policy, func() error {
		MutatePolicy(&policy, egressRules)
		return nil
	}); err != nil {
		return fmt.Errorf("failed to create / update NetworkPolicy %q in namespace %q: %w", AllowToSeedAPIServer, namespace, err)
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
