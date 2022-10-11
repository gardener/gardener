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

package istio

import (
	"embed"
	"fmt"
	"path/filepath"
	"strings"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/operation/botanist/component/coredns"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/nodelocaldns"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnauthzserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	istioIngressGatewayServicePortNameStatus = "status-port"
)

var (
	//go:embed charts/istio/istio-ingress
	chartIngress     embed.FS
	chartPathIngress = filepath.Join("charts", "istio", "istio-ingress")
)

// IngressGateway is a set of configuration values for the istio-ingress chart.
type IngressGateway struct {
	Values    IngressValues
	Namespace string
}

// IngressValues holds values for the istio-ingress chart.
// The only opened port is 15021.
type IngressValues struct {
	TrustDomain          string            `json:"trustDomain,omitempty"`
	Image                string            `json:"image,omitempty"`
	Annotations          map[string]string `json:"annotations,omitempty"`
	IstiodNamespace      string            `json:"istiodNamespace,omitempty"`
	LoadBalancerIP       *string           `json:"loadBalancerIP,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
	// Ports is a list of all Ports the istio-ingress gateways is listening on.
	// Port 15021 and 15000 cannot be used.
	Ports []corev1.ServicePort `json:"ports,omitempty"`
}

func (i *istiod) generateIstioIngressGatewayChart() (*chartrenderer.RenderedChart, error) {
	renderedChart := &chartrenderer.RenderedChart{}

	for _, istioIngressGateway := range i.istioIngressGatewayValues {
		values := map[string]interface{}{
			"trustDomain":       istioIngressGateway.Values.TrustDomain,
			"labels":            istioIngressGateway.Values.Labels,
			"annotations":       istioIngressGateway.Values.Annotations,
			"deployNamespace":   false,
			"priorityClassName": "istio-ingressgateway",
			"ports":             istioIngressGateway.Values.Ports,
			"image":             istioIngressGateway.Values.Image,
			"istiodNamespace":   istioIngressGateway.Values.IstiodNamespace,
			"loadBalancerIP":    istioIngressGateway.Values.LoadBalancerIP,
			"serviceName":       v1beta1constants.DefaultSNIIngressServiceName,
			"portsNames": map[string]interface{}{
				"status": istioIngressGatewayServicePortNameStatus,
			},
		}

		renderedIngressChart, err := i.chartRenderer.RenderEmbeddedFS(chartIngress, chartPathIngress, ManagedResourceControlName, istioIngressGateway.Namespace, values)
		if err != nil {
			return nil, err
		}

		addSuffixToManifestsName(renderedIngressChart, istioIngressGateway.Namespace)

		renderedChart.ChartName = renderedIngressChart.ChartName
		renderedChart.Manifests = append(renderedChart.Manifests, renderedIngressChart.Manifests...)
	}

	return renderedChart, nil
}

func getIngressGatewayNamespaceLabels(labels map[string]string) map[string]string {
	var namespaceLabels = map[string]string{
		"istio-operator-managed": "Reconcile",
		"istio-injection":        "disabled",
	}

	if value, ok := labels[v1beta1constants.GardenRole]; ok && value == v1beta1constants.GardenRoleExposureClassHandler {
		namespaceLabels[v1beta1constants.GardenRole] = v1beta1constants.GardenRoleExposureClassHandler
	}
	if value, ok := labels[v1beta1constants.LabelExposureClassHandlerName]; ok {
		namespaceLabels[v1beta1constants.LabelExposureClassHandlerName] = value
	}

	return namespaceLabels
}

func addSuffixToManifestsName(charts *chartrenderer.RenderedChart, suffix string) {
	for i := 0; i < len(charts.Manifests); i++ {
		charts.Manifests[i].Name = strings.TrimSuffix(charts.Manifests[i].Name, ".yaml")
		charts.Manifests[i].Name = charts.Manifests[i].Name + "/" + suffix + ".yaml"
	}
}

// IstioIngressNetworkPolicyValues contains deployment parameters for the istio-ingress network policies.
type IstioIngressNetworkPolicyValues struct {
	// NodeLocalIPVSAddress is the CIDR of the node-local IPVS address.
	NodeLocalIPVSAddress *string
	// DNSServerAddress is the CIDR of the usual DNS server address.
	DNSServerAddress *string
}

type networkPolicyTransformer struct {
	name      string
	transform func(*networkingv1.NetworkPolicy) func() error
}

func getIstioNetworkPolicyTransformers(values IstioIngressNetworkPolicyValues) []networkPolicyTransformer {
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
			name: "to-shoot-apiserver",
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
								Protocol: protocolPtr(corev1.ProtocolTCP),
								Port:     intStrPtr(kubeapiserver.Port),
							}},
						}},
					}
					return nil
				}
			},
		},
		{
			name: "to-shoot-vpn-seed-server",
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
								Protocol: protocolPtr(corev1.ProtocolTCP),
								Port:     intStrPtr(vpnseedserver.OpenVPNPort),
							}},
						}},
					}
					return nil
				}
			},
		},
		{
			name: "to-reversed-vpn-auth-server",
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
								Protocol: protocolPtr(corev1.ProtocolTCP),
								Port:     intStrPtr(vpnauthzserver.ServerPort),
							}},
						}},
					}
					return nil
				}
			},
		},
		{
			name: "to-istiod",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Egress from pods labeled with "+
							"'%s=%s' to pods labeled with '%s=%s' in namespace %s.",
							v1beta1constants.LabelApp, v1beta1constants.DefaultIngressGatewayAppLabelValue,
							v1beta1constants.LabelApp, "istiod",
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
										v1beta1constants.LabelApp: "istiod",
									},
								},
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"kubernetes.io/metadata.name": v1beta1constants.IstioSystemNamespace,
									},
								},
							}},
							Ports: []networkingv1.NetworkPolicyPort{{
								Protocol: protocolPtr(corev1.ProtocolTCP),
								Port:     intStrPtr(15012),
							}},
						}},
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
							"'%s=%s' to DNS running in '%s'.", v1beta1constants.LabelApp, v1beta1constants.DefaultIngressGatewayAppLabelValue,
							metav1.NamespaceSystem),
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelApp: v1beta1constants.DefaultIngressGatewayAppLabelValue,
							},
						},

						Egress: []networkingv1.NetworkPolicyEgressRule{{
							To: []networkingv1.NetworkPolicyPeer{
								{
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
								},
								{
									NamespaceSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											v1beta1constants.LabelRole: metav1.NamespaceSystem,
										},
									},
									PodSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{{
											Key:      coredns.LabelKey,
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{nodelocaldns.LabelValue},
										}},
									},
								},
							},
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
	}
}

func protocolPtr(protocol corev1.Protocol) *corev1.Protocol {
	return &protocol
}

func intStrPtr(port int) *intstr.IntOrString {
	v := intstr.FromInt(port)
	return &v
}
