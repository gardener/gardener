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

package networkpolicies

import (
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// GlobalValues contains deployment parameters for the global network policies.
type GlobalValues struct {
	// SNIEnabled states whether the SNI for kube-apiservers of shoot clusters is enabled.
	SNIEnabled bool
	// PrivateNetworkPeers is the list of peers for the private networks.
	PrivateNetworkPeers []networkingv1.NetworkPolicyPeer
	// DenyAllTraffic states whether all traffic should be denied by default and must be explicitly allowed by dedicated
	// network policy rules.
	DenyAllTraffic bool
}

type networkPolicyTransformer struct {
	name      string
	transform func(*networkingv1.NetworkPolicy) func() error
}

func getGlobalNetworkPolicyTransformers(values GlobalValues, isShootNamespace bool) []networkPolicyTransformer {
	result := []networkPolicyTransformer{
		{
			name: "deny-all",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: "Disables all Ingress and Egress traffic into/from this " +
							"namespace.",
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelNetworkPolicyToAll: v1beta1constants.LabelNetworkPolicyDisallowed,
							},
						},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress, networkingv1.PolicyTypeIngress},
					}

					if values.DenyAllTraffic {
						obj.Spec.PodSelector = metav1.LabelSelector{}
					}

					return nil
				}
			},
		},

		{
			name: "allow-to-private-networks",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Egress from pods labeled with "+
							"'%s=%s' to the Private networks (RFC1918), Carrier-grade NAT (RFC6598) except for "+
							"(1) CloudProvider's specific metadata service IP, (2) Seed networks, (3) Shoot networks",
							v1beta1constants.LabelNetworkPolicyToPrivateNetworks, v1beta1constants.LabelNetworkPolicyAllowed),
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
							},
						},
						Egress: []networkingv1.NetworkPolicyEgressRule{{
							To: values.PrivateNetworkPeers,
						}},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
					}
					return nil
				}
			},
		},
	}

	if !isShootNamespace {
		result = append(result,
			networkPolicyTransformer{
				name: "allow-to-aggregate-prometheus",
				transform: func(obj *networkingv1.NetworkPolicy) func() error {
					return func() error {
						obj.Annotations = map[string]string{
							v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Egress from pods labeled with "+
								"'%s=%s' to the aggregate-prometheus.", v1beta1constants.LabelNetworkPolicyToAggregatePrometheus,
								v1beta1constants.LabelNetworkPolicyAllowed),
						}
						obj.Spec = networkingv1.NetworkPolicySpec{
							PodSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1constants.LabelNetworkPolicyToAggregatePrometheus: v1beta1constants.LabelNetworkPolicyAllowed,
								},
							},
							Egress: []networkingv1.NetworkPolicyEgressRule{{
								To: []networkingv1.NetworkPolicyPeer{{
									NamespaceSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											v1beta1constants.LabelRole: v1beta1constants.GardenRoleGarden,
										},
									},
									PodSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											v1beta1constants.LabelApp:  "aggregate-prometheus",
											v1beta1constants.LabelRole: v1beta1constants.LabelMonitoring,
										},
									},
								}},
							}},
							PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
						}
						return nil
					}
				},
			},

			networkPolicyTransformer{
				name: "allow-to-seed-prometheus",
				transform: func(obj *networkingv1.NetworkPolicy) func() error {
					return func() error {
						obj.Annotations = map[string]string{
							v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Egress from pods labeled with "+
								"'%s=%s' to the seed-prometheus.", v1beta1constants.LabelNetworkPolicyToSeedPrometheus,
								v1beta1constants.LabelNetworkPolicyAllowed),
						}
						obj.Spec = networkingv1.NetworkPolicySpec{
							PodSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1constants.LabelNetworkPolicyToSeedPrometheus: v1beta1constants.LabelNetworkPolicyAllowed,
								},
							},
							Egress: []networkingv1.NetworkPolicyEgressRule{{
								To: []networkingv1.NetworkPolicyPeer{{
									NamespaceSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											v1beta1constants.LabelRole: v1beta1constants.GardenRoleGarden,
										},
									},
									PodSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											v1beta1constants.LabelApp:  "seed-prometheus",
											v1beta1constants.LabelRole: v1beta1constants.LabelMonitoring,
										},
									},
								}},
							}},
							PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
						}
						return nil
					}
				},
			},

			networkPolicyTransformer{
				name: "allow-to-all-shoot-apiservers",
				transform: func(obj *networkingv1.NetworkPolicy) func() error {
					return func() error {
						obj.Annotations = map[string]string{
							v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Egress from pods labeled with "+
								"'%s=%s' to all the shoot apiservers running in the seed cluster.",
								v1beta1constants.LabelNetworkPolicyToAllShootAPIServers, v1beta1constants.LabelNetworkPolicyAllowed),
						}
						obj.Spec = networkingv1.NetworkPolicySpec{
							PodSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1constants.LabelNetworkPolicyToAllShootAPIServers: v1beta1constants.LabelNetworkPolicyAllowed,
								},
							},
							Egress: []networkingv1.NetworkPolicyEgressRule{{
								To: []networkingv1.NetworkPolicyPeer{{
									NamespaceSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot,
										},
									},
									PodSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
											v1beta1constants.LabelApp:   v1beta1constants.LabelKubernetes,
											v1beta1constants.LabelRole:  v1beta1constants.LabelAPIServer,
										},
									},
								}},
							}},
							PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
						}

						if values.SNIEnabled {
							obj.Spec.Egress[0].To = append(obj.Spec.Egress[0].To, networkingv1.NetworkPolicyPeer{
								// we don't want to modify existing labels on the istio namespace
								NamespaceSelector: &metav1.LabelSelector{},
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.LabelApp: v1beta1constants.DefaultIngressGatewayAppLabelValue,
									},
								},
							})
						}

						return nil
					}
				},
			},
		)
	}

	return result
}
