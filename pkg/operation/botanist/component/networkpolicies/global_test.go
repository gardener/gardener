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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func constructNPAllowToAggregatePrometheus(namespace string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "allow-to-aggregate-prometheus",
			Namespace:   namespace,
			Annotations: map[string]string{"gardener.cloud/description": "Allows Egress from pods labeled with 'networking.gardener.cloud/to-aggregate-prometheus=allowed' to the aggregate-prometheus."},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"networking.gardener.cloud/to-aggregate-prometheus": "allowed"},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				To: []networkingv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"role": "garden",
						},
					},
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":  "aggregate-prometheus",
							"role": "monitoring",
						},
					},
				}},
			}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		},
	}
}

func constructNPAllowToSeedPrometheus(namespace string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "allow-to-seed-prometheus",
			Namespace:   namespace,
			Annotations: map[string]string{"gardener.cloud/description": "Allows Egress from pods labeled with 'networking.gardener.cloud/to-seed-prometheus=allowed' to the seed-prometheus."},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"networking.gardener.cloud/to-seed-prometheus": "allowed"},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				To: []networkingv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"role": "garden",
						},
					},
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":  "seed-prometheus",
							"role": "monitoring",
						},
					},
				}},
			}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		},
	}
}

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

func constructNPAllowToBlockedCIDRs(namespace string, blockedAddresses []string) *networkingv1.NetworkPolicy {
	obj := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "allow-to-blocked-cidrs",
			Namespace:   namespace,
			Annotations: map[string]string{"gardener.cloud/description": "Allows Egress from pods labeled with 'networking.gardener.cloud/to-blocked-cidrs=allowed' to CloudProvider's specific metadata service IP."},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"networking.gardener.cloud/to-blocked-cidrs": "allowed"},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		},
	}

	for _, address := range blockedAddresses {
		obj.Spec.Egress = append(obj.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{{
				IPBlock: &networkingv1.IPBlock{
					CIDR: address,
				},
			}},
		})
	}

	return obj
}

func constructNPAllowToDNS(namespace string, dnsServerAddress, nodeLocalIPVSAddress *string) *networkingv1.NetworkPolicy {
	var (
		protocolUDP = corev1.ProtocolUDP
		protocolTCP = corev1.ProtocolTCP
		port53      = intstr.FromInt(53)
		port8053    = intstr.FromInt(8053)

		obj = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "allow-to-dns",
				Namespace:   namespace,
				Annotations: map[string]string{"gardener.cloud/description": "Allows Egress from pods labeled with 'networking.gardener.cloud/to-dns=allowed' to DNS running in 'kube-system'. In practice, most of the Pods which require network Egress need this label."},
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"networking.gardener.cloud/to-dns": "allowed"},
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{{
					To: []networkingv1.NetworkPolicyPeer{{
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"role": "kube-system",
							},
						},
						PodSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{{
								Key:      "k8s-app",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"kube-dns"},
							}},
						},
					}},
					Ports: []networkingv1.NetworkPolicyPort{
						{Protocol: &protocolUDP, Port: &port53},
						{Protocol: &protocolTCP, Port: &port53},
						{Protocol: &protocolUDP, Port: &port8053},
						{Protocol: &protocolTCP, Port: &port8053},
					},
				}},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			},
		}
	)

	if dnsServerAddress != nil {
		obj.Spec.Egress[0].To = append(obj.Spec.Egress[0].To, networkingv1.NetworkPolicyPeer{
			IPBlock: &networkingv1.IPBlock{
				CIDR: fmt.Sprintf("%s/32", *dnsServerAddress),
			},
		})
	}

	if nodeLocalIPVSAddress != nil {
		obj.Spec.Egress[0].To = append(obj.Spec.Egress[0].To, networkingv1.NetworkPolicyPeer{
			IPBlock: &networkingv1.IPBlock{
				CIDR: fmt.Sprintf("%s/32", *nodeLocalIPVSAddress),
			},
		})
	}

	return obj
}

func constructNPDenyAll(namespace string, denyAllTraffic bool) *networkingv1.NetworkPolicy {
	obj := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "deny-all",
			Namespace:   namespace,
			Annotations: map[string]string{"gardener.cloud/description": "Disables all Ingress and Egress traffic into/from this namespace."},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"networking.gardener.cloud/to-all": "disallowed"},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress, networkingv1.PolicyTypeIngress},
		},
	}

	if denyAllTraffic {
		obj.Spec.PodSelector = metav1.LabelSelector{}
	}

	return obj
}

func constructNPAllowToPrivateNetworks(namespace string, peers []networkingv1.NetworkPolicyPeer) *networkingv1.NetworkPolicy {
	obj := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "allow-to-private-networks",
			Namespace:   namespace,
			Annotations: map[string]string{"gardener.cloud/description": "Allows Egress from pods labeled with 'networking.gardener.cloud/to-private-networks=allowed' to the Private networks (RFC1918), Carrier-grade NAT (RFC6598) except for (1) CloudProvider's specific metadata service IP, (2) Seed networks, (3) Shoot networks"},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"networking.gardener.cloud/to-private-networks": "allowed"},
			},
			Egress:      []networkingv1.NetworkPolicyEgressRule{{}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		},
	}

	if peers != nil {
		obj.Spec.Egress[0].To = peers
	}

	return obj
}

func constructNPAllowToPublicNetworks(namespace string, blockedAddresses []string) *networkingv1.NetworkPolicy {
	obj := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "allow-to-public-networks",
			Namespace:   namespace,
			Annotations: map[string]string{"gardener.cloud/description": "Allows Egress from pods labeled with 'networking.gardener.cloud/to-public-networks=allowed' to all Public network IPs, except for (1) Private networks (RFC1918), (2) Carrier-grade NAT (RFC6598), (3) CloudProvider's specific metadata service IP. In practice, this blocks Egress traffic to all networks in the Seed cluster and only traffic to public IPv4 addresses."},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"networking.gardener.cloud/to-public-networks": "allowed"},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				To: []networkingv1.NetworkPolicyPeer{{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "0.0.0.0/0",
						Except: []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "100.64.0.0/10"},
					},
				}},
			}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		},
	}

	obj.Spec.Egress[0].To[0].IPBlock.Except = append(obj.Spec.Egress[0].To[0].IPBlock.Except, blockedAddresses...)

	return obj
}
