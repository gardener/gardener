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
	"context"
	"embed"
	"fmt"
	"path/filepath"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/coredns"
	"github.com/gardener/gardener/pkg/operation/botanist/component/nodelocaldns"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	networkingv1 "k8s.io/api/networking/v1"
)

const (
	// ManagedResourceControlName is the name of the ManagedResource containing the resource specifications.
	ManagedResourceControlName = "istio"

	istiodServiceName            = "istiod"
	istiodServicePortNameMetrics = "metrics"
)

var (
	//go:embed charts/istio/istio-istiod
	chartIstiod     embed.FS
	chartPathIstiod = filepath.Join("charts", "istio", "istio-istiod")
)

type istiod struct {
	client                    crclient.Client
	chartRenderer             chartrenderer.Interface
	namespace                 string
	values                    IstiodValues
	istioIngressGatewayValues []IngressGateway
	istioProxyProtocolValues  []ProxyProtocol
}

// IstiodValues holds values for the istio-istiod chart.
type IstiodValues struct {
	TrustDomain string `json:"trustDomain,omitempty"`
	Image       string `json:"image,omitempty"`
	NodeLocalIPVSAddress *string           `json:"nodeLocalIPVSAddress,omitempty"`
	DNSServerAddress     *string           `json:"dnsServerAddress,omitempty"`
}

// NewIstio can be used to deploy istio's istiod in a namespace.
// Destroy does nothing.
func NewIstio(
	client crclient.Client,
	chartRenderer chartrenderer.Interface,
	values IstiodValues,
	namespace string,
	istioIngressGatewayValues []IngressGateway,
	istioProxyProtocolValues []ProxyProtocol,
) component.DeployWaiter {
	return &istiod{
		client:                    client,
		chartRenderer:             chartRenderer,
		values:                    values,
		namespace:                 namespace,
		istioIngressGatewayValues: istioIngressGatewayValues,
		istioProxyProtocolValues:  istioProxyProtocolValues,
	}
}

func (i *istiod) Deploy(ctx context.Context) error {
	if err := i.client.Create(
		ctx,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: i.namespace,
				Labels: map[string]string{
					"istio-operator-managed": "Reconcile",
					"istio-injection":        "disabled",
				},
			},
		},
	); client.IgnoreAlreadyExists(err) != nil {
		return err
	}

	// TODO(mvladev): Rotate this on every istio version upgrade.
	for _, filterName := range []string{"tcp-stats-filter-1.10", "stats-filter-1.10"} {
		if err := crclient.IgnoreNotFound(i.client.Delete(ctx, &networkingv1alpha3.EnvoyFilter{
			ObjectMeta: metav1.ObjectMeta{Name: filterName, Namespace: i.namespace},
		})); err != nil {
			return err
		}
	}

	renderedIstiodChart, err := i.generateIstiodChart()
	if err != nil {
		return err
	}

	renderedIstioIngressGatewayChart, err := i.generateIstioIngressGatewayChart()
	if err != nil {
		return err
	}

	renderedIstioProxyProtocolChart, err := i.generateIstioProxyProtocolChart()
	if err != nil {
		return err
	}

	for _, istioIngressGateway := range i.istioIngressGatewayValues {
		if err := i.client.Create(
			ctx,
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   istioIngressGateway.Namespace,
					Labels: getIngressGatewayNamespaceLabels(istioIngressGateway.Values.Labels),
				},
			},
		); client.IgnoreAlreadyExists(err) != nil {
			return err
		}
	}

	renderedChart := renderedIstiodChart
	renderedChart.Manifests = append(renderedChart.Manifests, renderedIstioIngressGatewayChart.Manifests...)
	if gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI) {
		renderedChart.Manifests = append(renderedChart.Manifests, renderedIstioProxyProtocolChart.Manifests...)
	}
	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	
	
	for _, transformer := range getIstiodNetworkPolicyTransformers(
		IstiodNetworkPolicyValues{
		APIServerAddress: "52.211.31.126/32",
		NodeLocalIPVSAddress: i.values.NodeLocalIPVSAddress,
		DNSServerAddress:     i.values.DNSServerAddress,
		}) {
		obj := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      transformer.name,
				Namespace: v1beta1constants.IstioSystemNamespace,
			},
		}

		if err := transformer.transform(obj)(); err != nil {
			return err
		}

		if err := registry.Add(obj); err != nil {
			return err
		}
	}
		
	for _, istioIngressGateway := range i.istioIngressGatewayValues {
		for _, transformer := range getIstioNetworkPolicyTransformers(
			IstioIngressNetworkPolicyValues{
				NodeLocalIPVSAddress: i.values.NodeLocalIPVSAddress,
				DNSServerAddress:     i.values.DNSServerAddress,
			}) {
			obj := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      transformer.name,
					Namespace: istioIngressGateway.Namespace,
				},
			}

			if err := transformer.transform(obj)(); err != nil {
				return err
			}

			if err := registry.Add(obj); err != nil {
				return err
			}
		}
	}

	chartsMap := renderedChart.AsSecretData()
	objMap := registry.SerializedObjects()

	for key := range objMap {
		chartsMap[key] = objMap[key]
	}
	
	return managedresources.CreateForSeed(ctx, i.client, i.namespace, ManagedResourceControlName, false, chartsMap)
}

func (i *istiod) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForSeed(ctx, i.client, i.namespace, ManagedResourceControlName); err != nil {
		return err
	}

	if err := i.client.Delete(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: i.namespace,
		},
	}); crclient.IgnoreNotFound(err) != nil {
		return err
	}

	for _, istioIngressGateway := range i.istioIngressGatewayValues {
		if err := i.client.Delete(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: istioIngressGateway.Namespace,
			},
		}); crclient.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (i *istiod) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, i.client, i.namespace, ManagedResourceControlName)
}

func (i *istiod) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, i.client, i.namespace, ManagedResourceControlName)
}

func (i *istiod) generateIstiodChart() (*chartrenderer.RenderedChart, error) {
	return i.chartRenderer.RenderEmbeddedFS(chartIstiod, chartPathIstiod, ManagedResourceControlName, i.namespace, map[string]interface{}{
		"serviceName": istiodServiceName,
		"trustDomain": i.values.TrustDomain,
		"labels": map[string]interface{}{
			"app":   "istiod",
			"istio": "pilot",
		},
		"deployNamespace":   false,
		"priorityClassName": "istiod",
		"ports": map[string]interface{}{
			"https": 10250,
		},
		"portsNames": map[string]interface{}{
			"metrics": istiodServicePortNameMetrics,
		},
		"image": i.values.Image,
	})
}


type IstiodNetworkPolicyValues struct {
	APIServerAddress string
	// NodeLocalIPVSAddress is the CIDR of the node-local IPVS address.
	NodeLocalIPVSAddress *string
	// DNSServerAddress is the CIDR of the usual DNS server address.
	DNSServerAddress *string
}

func getIstiodNetworkPolicyTransformers(values IstiodNetworkPolicyValues) []networkPolicyTransformer {
	return []networkPolicyTransformer{
		{
			name: "deny-all",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: "Disables all Ingress and Egress traffic into/from this " +
							"namespace.",
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress, networkingv1.PolicyTypeIngress},
					}

					return nil
				}
			},
		},
		{
			name: "allow-to-seed-apiserver",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: "Allow egress to seed api-server.",
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelApp: "istiod",
							},
						},
						Egress: []networkingv1.NetworkPolicyEgressRule{{
							To: []networkingv1.NetworkPolicyPeer{
								{
									IPBlock: &networkingv1.IPBlock{CIDR: values.APIServerAddress}},	
								},
							Ports: []networkingv1.NetworkPolicyPort{
									{Protocol: protocolPtr(corev1.ProtocolTCP), Port: intStrPtr(443)},
								},
							},
						},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
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
		{
			name: "allow-from-vpn",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: "Allow ingress from vpn.",
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelApp: "istiod",
							},
						},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
						Ingress: []networkingv1.NetworkPolicyIngressRule{{
							From: []networkingv1.NetworkPolicyPeer{{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.LabelApp:   "vpn-shoot",
										v1beta1constants.GardenRole: "system-component",
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
		{
			name: "allow-from-aggregate-prometheus",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: "Allow ingress from aggregate-prometheus.",
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelApp: "istiod",
							},
						},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
						Ingress: []networkingv1.NetworkPolicyIngressRule{{
							From: []networkingv1.NetworkPolicyPeer{{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.LabelApp:   "aggregate-prometheus",
										v1beta1constants.GardenRole: "monitoring",
									},
								},
								NamespaceSelector: &metav1.LabelSelector{},
								}},
								Ports: []networkingv1.NetworkPolicyPort{{
									Protocol: protocolPtr(corev1.ProtocolTCP), 
									Port:  func (port string) *intstr.IntOrString {v := intstr.FromString(port); return &v } ("metrics"),
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
						v1beta1constants.GardenerDescription: "Allow ingress from istio-ingress.",
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelApp: "istiod",
							},
						},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
						Ingress: []networkingv1.NetworkPolicyIngressRule{{
							From: []networkingv1.NetworkPolicyPeer{{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.LabelApp:   "istio-ingressgateway",
									},
								},
								NamespaceSelector: &metav1.LabelSelector{},
							}},
							// Ports: []networkingv1.NetworkPolicyPort{{
							// 		Protocol: protocolPtr(corev1.ProtocolTCP), Port:  intstr.FromString("metrics")
							// 	}},
							},
						},
					}
					return nil
				}
			},
		},
	}
}
