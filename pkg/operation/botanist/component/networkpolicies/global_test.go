// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package networkpolicies_test

import (
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func constructNPAllowToAllShootAPIServers(namespace string, sniEnabled bool) *networkingv1.NetworkPolicy {
	obj := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "allow-to-all-shoot-apiservers",
			Namespace:   namespace,
			Annotations: map[string]string{"gardener.cloud/description": "Allows Egress from pods labeled with 'networking.gardener.cloud/to-all-shoot-apiservers=allowed' to all the shoot apiservers running in the seed cluster."},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"networking.gardener.cloud/to-all-shoot-apiservers": "allowed"},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				To: []networkingv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"gardener.cloud/role": "shoot",
						},
					},
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"gardener.cloud/role": "controlplane",
							"app":                 "kubernetes",
							"role":                "apiserver",
						},
					},
				}},
			}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		},
	}

	if sniEnabled {
		obj.Spec.Egress[0].To = append(obj.Spec.Egress[0].To, networkingv1.NetworkPolicyPeer{
			NamespaceSelector: &metav1.LabelSelector{},
			PodSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "istio-ingressgateway",
				},
			},
		})
	}

	return obj
}
