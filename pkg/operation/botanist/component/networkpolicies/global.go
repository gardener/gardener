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

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component/coredns"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// GlobalValues contains deployment parameters for the global network policies.
type GlobalValues struct {
	// SNIEnabled states whether the SNI for kube-apiservers of shoot clusters is enabled.
	SNIEnabled bool
	// BlockedAddresses is a list of CIDRs that should be blocked from being accessed.
	BlockedAddresses []string
	// PrivateNetworkPeers is the list of peers for the private networks.
	PrivateNetworkPeers []networkingv1.NetworkPolicyPeer
	// DenyAllTraffic states whether all traffic should be denied by default and must be explicitly allowed by dedicated
	// network policy rules.
	DenyAllTraffic bool
	// NodeLocalIPVSAddress is the CIDR of the node-local IPVS address.
	NodeLocalIPVSAddress *string
	// DNSServerAddress is the CIDR of the usual DNS server address.
	DNSServerAddress *string
}

type networkPolicyTransformer struct {
	name      string
	transform func(*networkingv1.NetworkPolicy) func() error
}

func getGlobalNetworkPolicyTransformers(values GlobalValues) []networkPolicyTransformer {
	return []networkPolicyTransformer{
		{
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

		{
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

		{
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

		{
			name: "allow-to-blocked-cidrs",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Egress from pods labeled with "+
							"'%s=%s' to CloudProvider's specific metadata service IP.", v1beta1constants.LabelNetworkPolicyToBlockedCIDRs,
							v1beta1constants.LabelNetworkPolicyAllowed),
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelNetworkPolicyToBlockedCIDRs: v1beta1constants.LabelNetworkPolicyAllowed,
							},
						},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
					}

					for _, address := range values.BlockedAddresses {
						obj.Spec.Egress = append(obj.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
							To: []networkingv1.NetworkPolicyPeer{{
								IPBlock: &networkingv1.IPBlock{
									CIDR: address,
								},
							}},
						})
					}

					return nil
				}
			},
		},

		{
			name: "allow-to-dns",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Egress from pods labeled with "+
							"'%s=%s' to DNS running in '%s'. In practice, most of the Pods which require network Egress "+
							"need this label.", v1beta1constants.LabelNetworkPolicyToDNS, v1beta1constants.LabelNetworkPolicyAllowed,
							metav1.NamespaceSystem),
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelNetworkPolicyToDNS: v1beta1constants.LabelNetworkPolicyAllowed,
							},
						},
						Egress: []networkingv1.NetworkPolicyEgressRule{{
							To: []networkingv1.NetworkPolicyPeer{{
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.LabelRole: metav1.NamespaceSystem,
									},
								},
								PodSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      coredns.LabelKey,
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{coredns.LabelValue},
									}},
								},
							}},
							Ports: []networkingv1.NetworkPolicyPort{
								{Protocol: protocolPtr(corev1.ProtocolUDP), Port: intStrPtr(coredns.PortServiceServer)},
								{Protocol: protocolPtr(corev1.ProtocolTCP), Port: intStrPtr(coredns.PortServiceServer)},
								{Protocol: protocolPtr(corev1.ProtocolUDP), Port: intStrPtr(coredns.PortServer)},
								{Protocol: protocolPtr(corev1.ProtocolTCP), Port: intStrPtr(coredns.PortServer)},
							},
						}},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
					}

					if values.DNSServerAddress != nil {
						obj.Spec.Egress[0].To = append(obj.Spec.Egress[0].To, networkingv1.NetworkPolicyPeer{
							IPBlock: &networkingv1.IPBlock{
								// required for node local dns feature, allows egress traffic to CoreDNS
								CIDR: fmt.Sprintf("%s/32", *values.DNSServerAddress),
							},
						})
					}

					if values.NodeLocalIPVSAddress != nil {
						obj.Spec.Egress[0].To = append(obj.Spec.Egress[0].To, networkingv1.NetworkPolicyPeer{
							IPBlock: &networkingv1.IPBlock{
								// required for node local dns feature, allows egress traffic to node local dns cache
								CIDR: fmt.Sprintf("%s/32", *values.NodeLocalIPVSAddress),
							},
						})
					}

					return nil
				}
			},
		},

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

		{
			name: "allow-to-public-networks",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Egress from pods labeled with "+
							"'%s=%s' to all Public network IPs, except for (1) Private networks (RFC1918), "+
							"(2) Carrier-grade NAT (RFC6598), (3) CloudProvider's specific metadata service IP. In "+
							"practice, this blocks Egress traffic to all networks in the Seed cluster and only traffic "+
							"to public IPv4 addresses.", v1beta1constants.LabelNetworkPolicyToPublicNetworks,
							v1beta1constants.LabelNetworkPolicyAllowed),
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelNetworkPolicyToPublicNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
							},
						},
						Egress: []networkingv1.NetworkPolicyEgressRule{{
							To: []networkingv1.NetworkPolicyPeer{{
								IPBlock: &networkingv1.IPBlock{
									CIDR: "0.0.0.0/0",
									Except: append([]string{
										Private8BitBlock().String(),
										Private12BitBlock().String(),
										Private16BitBlock().String(),
										CarrierGradeNATBlock().String(),
									}, values.BlockedAddresses...),
								},
							}},
						}},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
					}

					return nil
				}
			},
		},
	}
}

func protocolPtr(protocol corev1.Protocol) *corev1.Protocol {
	return &protocol
}

func intStrPtr(port int) *intstr.IntOrString {
	v := intstr.FromInt(port)
	return &v
}
