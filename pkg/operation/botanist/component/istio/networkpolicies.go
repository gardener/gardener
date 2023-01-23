// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package istio

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnauthzserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnshoot"
	"github.com/gardener/gardener/pkg/utils"
)

const (
	istiodAppLabelValue = "istiod"
)

type networkPolicyTransformer struct {
	name      string
	transform func(*networkingv1.NetworkPolicy) func() error
}

func getIstioSystemNetworkPolicyTransformers() []networkPolicyTransformer {
	return []networkPolicyTransformer{
		{
			name: "allow-to-istiod-webhook-server-port",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: "Allows Ingress from all sources to the webhook server port of istiod",
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelApp: istiodAppLabelValue,
							},
						},
						Ingress: []networkingv1.NetworkPolicyIngressRule{{
							From: []networkingv1.NetworkPolicyPeer{
								{
									IPBlock: &networkingv1.IPBlock{
										CIDR: "0.0.0.0/0",
									},
								},
							},
							Ports: []networkingv1.NetworkPolicyPort{
								{Protocol: utils.ProtocolPtr(corev1.ProtocolTCP), Port: utils.IntStrPtrFromInt(portWebhookServer)},
							},
						}},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
					}

					return nil
				}
			},
		},
		{
			name: "allow-from-shoot-vpn",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Ingress from shoot vpn servers with label "+
							"'%s=%s'. It's needed to call the validating webhook istiod by the shoot apiserver.",
							v1beta1constants.LabelApp, vpnshoot.LabelValue),
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelApp: istiodAppLabelValue,
							},
						},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
						Ingress: []networkingv1.NetworkPolicyIngressRule{{
							From: []networkingv1.NetworkPolicyPeer{{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.LabelApp:   vpnshoot.LabelValue,
										v1beta1constants.GardenRole: v1beta1constants.GardenRoleSystemComponent,
									},
								},
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.LabelRole: metav1.NamespaceSystem,
									},
								},
							}},
						},
						},
					}
					return nil
				}
			},
		},
		{
			name: "allow-from-aggregate-prometheus",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Ingress from pods with label "+
							"'%s=%s'", v1beta1constants.LabelApp, "aggregate-prometheus"),
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelApp: istiodAppLabelValue,
							},
						},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
						Ingress: []networkingv1.NetworkPolicyIngressRule{{
							From: []networkingv1.NetworkPolicyPeer{{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.LabelApp:  "aggregate-prometheus",
										v1beta1constants.LabelRole: v1beta1constants.GardenRoleMonitoring,
									},
								},
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.LabelRole: v1beta1constants.GardenNamespace,
									},
								},
							}},
							Ports: []networkingv1.NetworkPolicyPort{{
								Protocol: utils.ProtocolPtr(corev1.ProtocolTCP),
								Port:     utils.IntStrPtrFromInt(15014),
							}},
						},
						},
					}
					return nil
				}
			},
		},
		{
			name: "allow-from-istio-ingress",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Ingress from pods with label "+
							"'%s=%s'", v1beta1constants.LabelApp, v1beta1constants.DefaultIngressGatewayAppLabelValue),
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelApp: istiodAppLabelValue,
							},
						},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
						Ingress: []networkingv1.NetworkPolicyIngressRule{{
							From: []networkingv1.NetworkPolicyPeer{{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.LabelApp: v1beta1constants.DefaultIngressGatewayAppLabelValue,
									},
								},
								NamespaceSelector: &metav1.LabelSelector{},
							}},
						},
						},
					}
					return nil
				}
			},
		},
	}
}

func getIstioIngressNetworkPolicyTransformers() []networkPolicyTransformer {
	return []networkPolicyTransformer{
		{
			name: "deny-all-egress",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: "Deny all egress traffic from pods in this namespace.",
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
					}
					return nil
				}
			},
		},
		{
			name: "allow-to-shoot-apiserver",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Egress from pods labeled with "+
							"'%s=%s' to shoot api servers with label '%s=%s'.", v1beta1constants.LabelApp, v1beta1constants.DefaultIngressGatewayAppLabelValue,
							v1beta1constants.LabelRole, v1beta1constants.LabelAPIServer),
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelApp: v1beta1constants.DefaultIngressGatewayAppLabelValue,
							},
						},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
						Egress: []networkingv1.NetworkPolicyEgressRule{{
							To: []networkingv1.NetworkPolicyPeer{{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.LabelApp:   v1beta1constants.LabelKubernetes,
										v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
										v1beta1constants.LabelRole:  v1beta1constants.LabelAPIServer,
									},
								},
								NamespaceSelector: &metav1.LabelSelector{},
							}},
							Ports: []networkingv1.NetworkPolicyPort{{
								Protocol: utils.ProtocolPtr(corev1.ProtocolTCP),
								Port:     utils.IntStrPtrFromInt(kubeapiserverconstants.Port),
							}},
						}},
					}
					return nil
				}
			},
		},
		{
			name: "allow-to-shoot-vpn-seed-server",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Egress from pods labeled with "+
							"'%s=%s' to shoot vpn servers with label '%s=%s'.", v1beta1constants.LabelApp, v1beta1constants.DefaultIngressGatewayAppLabelValue,
							v1beta1constants.LabelApp, v1beta1constants.DeploymentNameVPNSeedServer),
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelApp: v1beta1constants.DefaultIngressGatewayAppLabelValue,
							},
						},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
						Egress: []networkingv1.NetworkPolicyEgressRule{{
							To: []networkingv1.NetworkPolicyPeer{{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.LabelApp:   v1beta1constants.DeploymentNameVPNSeedServer,
										v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
									},
								},
								NamespaceSelector: &metav1.LabelSelector{},
							}},
							Ports: []networkingv1.NetworkPolicyPort{{
								Protocol: utils.ProtocolPtr(corev1.ProtocolTCP),
								Port:     utils.IntStrPtrFromInt(vpnseedserver.OpenVPNPort),
							}},
						}},
					}
					return nil
				}
			},
		},
		{
			name: "allow-to-reversed-vpn-auth-server",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Egress from pods labeled with "+
							"'%s=%s' to reversed vpn auth servers with label '%s=%s' in namespace %s.",
							v1beta1constants.LabelApp, v1beta1constants.DefaultIngressGatewayAppLabelValue,
							v1beta1constants.LabelApp, vpnauthzserver.Name,
							v1beta1constants.GardenNamespace,
						),
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelApp: v1beta1constants.DefaultIngressGatewayAppLabelValue,
							},
						},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
						Egress: []networkingv1.NetworkPolicyEgressRule{{
							To: []networkingv1.NetworkPolicyPeer{{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.LabelApp: vpnauthzserver.Name,
									},
								},
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.LabelRole: v1beta1constants.GardenNamespace,
									},
								},
							}},
							Ports: []networkingv1.NetworkPolicyPort{{
								Protocol: utils.ProtocolPtr(corev1.ProtocolTCP),
								Port:     utils.IntStrPtrFromInt(vpnauthzserver.ServerPort),
							}},
						}},
					}
					return nil
				}
			},
		},
		{
			name: "allow-to-istiod",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Egress from pods labeled with "+
							"'%s=%s' to pods labeled with '%s=%s' in namespace %s.",
							v1beta1constants.LabelApp, v1beta1constants.DefaultIngressGatewayAppLabelValue,
							v1beta1constants.LabelApp, istiodAppLabelValue,
							v1beta1constants.IstioSystemNamespace,
						),
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelApp: v1beta1constants.DefaultIngressGatewayAppLabelValue,
							},
						},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
						Egress: []networkingv1.NetworkPolicyEgressRule{{
							To: []networkingv1.NetworkPolicyPeer{{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.LabelApp: istiodAppLabelValue,
									},
								},
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"kubernetes.io/metadata.name": v1beta1constants.IstioSystemNamespace,
									},
								},
							}},
							Ports: []networkingv1.NetworkPolicyPort{{
								Protocol: utils.ProtocolPtr(corev1.ProtocolTCP),
								Port:     utils.IntStrPtrFromInt(15012),
							}},
						}},
					}
					return nil
				}
			},
		},
	}
}
