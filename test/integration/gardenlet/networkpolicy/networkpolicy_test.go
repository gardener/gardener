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

package networkpolicy_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	networkpolicyhelper "github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy/helper"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("NetworkPolicy controller tests", func() {
	var (
		gardenNamespace       *corev1.Namespace
		istioSystemNamespace  *corev1.Namespace
		istioIngressNamespace *corev1.Namespace
		shootNamespace        *corev1.Namespace
		fooNamespace          *corev1.Namespace
	)

	BeforeEach(func() {
		gardenNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "garden-",
				Labels: map[string]string{
					testID: testRunID,
					"role": v1beta1constants.GardenNamespace,
				},
			},
		}
		istioSystemNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "istio-system-",
				Labels: map[string]string{
					testID:                      testRunID,
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioSystem,
				},
			},
		}
		istioIngressNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "istio-ingress-",
				Labels: map[string]string{
					testID:                      testRunID,
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress,
				},
			},
		}
		shootNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "shoot--",
				Labels: map[string]string{
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot,
					testID:                      testRunID,
				},
			},
		}
		fooNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "foo--",
				Labels:       map[string]string{testID: testRunID},
			},
		}
	})

	JustBeforeEach(func() {
		By("Create garden namespace")
		Expect(testClient.Create(ctx, gardenNamespace)).To(Succeed())
		log.Info("Created garden namespace for test", "namespaceName", gardenNamespace.Name)

		DeferCleanup(func() {
			By("Delete garden Namespace")
			Expect(testClient.Delete(ctx, gardenNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Create istio-system namespace")
		Expect(testClient.Create(ctx, istioSystemNamespace)).To(Succeed())
		log.Info("Created istio-system namespace for test", "namespaceName", istioSystemNamespace.Name)

		DeferCleanup(func() {
			By("Delete istio-system Namespace")
			Expect(testClient.Delete(ctx, istioSystemNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Create istio-ingress namespace")
		Expect(testClient.Create(ctx, istioIngressNamespace)).To(Succeed())
		log.Info("Created istio-ingress namespace for test", "namespaceName", istioIngressNamespace.Name)

		DeferCleanup(func() {
			By("Delete istio-ingress Namespace")
			Expect(testClient.Delete(ctx, istioIngressNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Create shoot namespace")
		Expect(testClient.Create(ctx, shootNamespace)).To(Succeed())
		log.Info("Created shoot namespace for test", "namespace", client.ObjectKeyFromObject(shootNamespace))

		DeferCleanup(func() {
			By("Delete shoot namespace")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shootNamespace))).To(Succeed())
		})

		By("Create foo namespace")
		Expect(testClient.Create(ctx, fooNamespace)).To(Succeed())
		log.Info("Created foo namespace for test", "namespace", client.ObjectKeyFromObject(fooNamespace))

		DeferCleanup(func() {
			By("Delete foo namespace")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, fooNamespace))).To(Succeed())
		})
	})

	type testAttributes struct {
		networkPolicyName         string
		expectedNetworkPolicySpec func() networkingv1.NetworkPolicySpec
		inGardenNamespace         bool
		inIstioSystemNamespace    bool
		inIstioIngressNamespace   bool
		inShootNamespaces         bool
	}

	defaultTests := func(attrs testAttributes) {
		Context("reconciliation", func() {
			if attrs.inShootNamespaces {
				It("should create the network policy in the shoot namespace", func() {
					By("Wait for controller to reconcile the network policy")
					Eventually(func(g Gomega) {
						networkPolicy := &networkingv1.NetworkPolicy{}
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: shootNamespace.Name, Name: attrs.networkPolicyName}, networkPolicy)).To(Succeed())
						g.Expect(networkPolicy.Spec).To(Equal(attrs.expectedNetworkPolicySpec()))
					}).Should(Succeed())
				})
			} else {
				It("should not create the network policy in the shoot namespace", func() {
					By("Ensure controller does not reconcile the network policy")
					Consistently(func(g Gomega) {
						networkPolicy := &networkingv1.NetworkPolicy{}
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: shootNamespace.Name, Name: attrs.networkPolicyName}, networkPolicy)).Should(BeNotFoundError())
					}).Should(Succeed())
				})
			}

			if attrs.inGardenNamespace {
				It("should create the network policy in the garden namespace", func() {
					By("Wait for controller to reconcile the network policy")
					Eventually(func(g Gomega) {
						networkPolicy := &networkingv1.NetworkPolicy{}
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace.Name, Name: attrs.networkPolicyName}, networkPolicy)).To(Succeed())
						g.Expect(networkPolicy.Spec).To(Equal(attrs.expectedNetworkPolicySpec()))
					}).Should(Succeed())
				})
			} else {
				It("should not create the network policy in the garden namespace", func() {
					By("Ensure controller does not reconcile the network policy")
					Consistently(func(g Gomega) {
						networkPolicy := &networkingv1.NetworkPolicy{}
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace.Name, Name: attrs.networkPolicyName}, networkPolicy)).Should(BeNotFoundError())
					}).Should(Succeed())
				})
			}

			if attrs.inIstioSystemNamespace {
				It("should create the network policy in the istio-system namespace", func() {
					By("Wait for controller to reconcile the network policy")
					Eventually(func(g Gomega) {
						networkPolicy := &networkingv1.NetworkPolicy{}
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: istioSystemNamespace.Name, Name: attrs.networkPolicyName}, networkPolicy)).To(Succeed())
						g.Expect(networkPolicy.Spec).To(Equal(attrs.expectedNetworkPolicySpec()))
					}).Should(Succeed())
				})
			} else {
				It("should not create the network policy in the istio-system namespace", func() {
					By("Ensure controller does not reconcile the network policy")
					Consistently(func(g Gomega) {
						networkPolicy := &networkingv1.NetworkPolicy{}
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: istioSystemNamespace.Name, Name: attrs.networkPolicyName}, networkPolicy)).Should(BeNotFoundError())
					}).Should(Succeed())
				})
			}

			if attrs.inIstioIngressNamespace {
				It("should create the network policy in the istio-ingress namespace", func() {
					By("Wait for controller to reconcile the network policy")
					Eventually(func(g Gomega) {
						networkPolicy := &networkingv1.NetworkPolicy{}
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: istioIngressNamespace.Name, Name: attrs.networkPolicyName}, networkPolicy)).To(Succeed())
						g.Expect(networkPolicy.Spec).To(Equal(attrs.expectedNetworkPolicySpec()))
					}).Should(Succeed())
				})
			} else {
				It("should not create the network policy in the istio-ingress namespace", func() {
					By("Ensure controller does not reconcile the network policy")
					Consistently(func(g Gomega) {
						networkPolicy := &networkingv1.NetworkPolicy{}
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: istioIngressNamespace.Name, Name: attrs.networkPolicyName}, networkPolicy)).Should(BeNotFoundError())
					}).Should(Succeed())
				})
			}

			It("should not create the network policy in the foo namespace", func() {
				By("Ensure controller does not reconcile the network policy")
				Consistently(func(g Gomega) {
					networkPolicy := &networkingv1.NetworkPolicy{}
					g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: fooNamespace.Name, Name: attrs.networkPolicyName}, networkPolicy)).Should(BeNotFoundError())
				}).Should(Succeed())
			})

			It("should reconcile the network policy when it is changed by a third party", func() {
				By("Modify network policy")
				modifiedShootNetworkPolicy := &networkingv1.NetworkPolicy{}
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace.Name, Name: attrs.networkPolicyName}, modifiedShootNetworkPolicy)
				}).Should(Succeed())
				modifiedShootNetworkPolicy.Spec.PodSelector.MatchLabels["foo"] = "bar"
				Expect(testClient.Update(ctx, modifiedShootNetworkPolicy)).To(Succeed())

				By("Wait for controller to reconcile the network policy")
				Eventually(func(g Gomega) {
					shootNetworkPolicy := &networkingv1.NetworkPolicy{}
					g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace.Name, Name: attrs.networkPolicyName}, shootNetworkPolicy)).To(Succeed())
					g.Expect(shootNetworkPolicy.Spec).To(Equal(attrs.expectedNetworkPolicySpec()))
				}).Should(Succeed())
			})

			It("should not update the network policy if nothing changed", func() {
				By("Modify namespace to trigger reconciliation")
				beforeShootNetworkPolicy := &networkingv1.NetworkPolicy{}
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace.Name, Name: attrs.networkPolicyName}, beforeShootNetworkPolicy)
				}).Should(Succeed())
				gardenNamespace.Labels["foo"] = "bar"
				Expect(testClient.Update(ctx, gardenNamespace)).To(Succeed())

				By("Wait for controller to reconcile the network policy")
				Consistently(func(g Gomega) {
					shootNetworkPolicy := &networkingv1.NetworkPolicy{}
					g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace.Name, Name: attrs.networkPolicyName}, shootNetworkPolicy)).To(Succeed())
					g.Expect(shootNetworkPolicy.ResourceVersion).To(Equal(beforeShootNetworkPolicy.ResourceVersion))
				}).Should(Succeed())
			})
		})

		Context("deletion", func() {
			It("should delete the network policy in foo namespace", func() {
				networkPolicy := &networkingv1.NetworkPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: fooNamespace.Name,
						Name:      attrs.networkPolicyName,
					},
					Spec: attrs.expectedNetworkPolicySpec(),
				}

				By("Create network policy")
				Expect(testClient.Create(ctx, networkPolicy)).To(Succeed())

				DeferCleanup(func() {
					By("Delete network policy")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, networkPolicy))).To(Succeed())
				})

				By("Wait for controller to delete the network policy")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).Should(BeNotFoundError())
				}).Should(Succeed())
			})
		})
	}

	Describe("allow-to-{seed,runtime}-apiserver", func() {
		tests := func(networkPolicyName, labelKey string) {
			var expectedNetworkPolicySpec networkingv1.NetworkPolicySpec

			JustBeforeEach(func() {
				kubernetesEndpoint := &corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "kubernetes"}}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(kubernetesEndpoint), kubernetesEndpoint)).To(Succeed())

				expectedNetworkPolicySpec = networkingv1.NetworkPolicySpec{
					Egress:      networkpolicyhelper.GetEgressRules(kubernetesEndpoint.Subsets...),
					PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{labelKey: "allowed"}},
					PolicyTypes: []networkingv1.PolicyType{"Egress"},
				}
			})

			defaultTests(testAttributes{
				networkPolicyName:         networkPolicyName,
				expectedNetworkPolicySpec: func() networkingv1.NetworkPolicySpec { return expectedNetworkPolicySpec },
				inGardenNamespace:         true,
				inIstioSystemNamespace:    true,
				inShootNamespaces:         true,
			})
		}

		tests("allow-to-seed-apiserver", "networking.gardener.cloud/to-seed-apiserver")
		tests("allow-to-runtime-apiserver", "networking.gardener.cloud/to-runtime-apiserver")
	})

	Describe("allow-to-public-networks", func() {
		var (
			networkPolicyName         = "allow-to-public-networks"
			expectedNetworkPolicySpec = networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyToPublicNetworks: v1beta1constants.LabelNetworkPolicyAllowed}},
				Egress: []networkingv1.NetworkPolicyEgressRule{{
					To: []networkingv1.NetworkPolicyPeer{{
						IPBlock: &networkingv1.IPBlock{
							CIDR: "0.0.0.0/0",
							Except: []string{
								"10.0.0.0/8",
								"172.16.0.0/12",
								"192.168.0.0/16",
								"100.64.0.0/10",
								blockedCIDR,
							},
						},
					}},
				}},
				PolicyTypes: []networkingv1.PolicyType{"Egress"},
			}
		)

		defaultTests(testAttributes{
			networkPolicyName:         networkPolicyName,
			expectedNetworkPolicySpec: func() networkingv1.NetworkPolicySpec { return expectedNetworkPolicySpec },
			inGardenNamespace:         true,
			inShootNamespaces:         true,
		})
	})

	Describe("allow-to-blocked-cidrs", func() {
		var (
			networkPolicyName         = "allow-to-blocked-cidrs"
			expectedNetworkPolicySpec = networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyToBlockedCIDRs: v1beta1constants.LabelNetworkPolicyAllowed}},
				PolicyTypes: []networkingv1.PolicyType{"Egress"},
				Egress:      []networkingv1.NetworkPolicyEgressRule{{To: []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: blockedCIDR}}}}},
			}
		)

		defaultTests(testAttributes{
			networkPolicyName:         networkPolicyName,
			expectedNetworkPolicySpec: func() networkingv1.NetworkPolicySpec { return expectedNetworkPolicySpec },
			inGardenNamespace:         true,
			inShootNamespaces:         true,
		})
	})

	Describe("allow-to-dns", func() {
		defaultTests(testAttributes{
			networkPolicyName: "allow-to-dns",
			expectedNetworkPolicySpec: func() networkingv1.NetworkPolicySpec {
				return networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyToDNS: v1beta1constants.LabelNetworkPolicyAllowed}},
					PolicyTypes: []networkingv1.PolicyType{"Egress"},
					Egress: []networkingv1.NetworkPolicyEgressRule{{
						To: []networkingv1.NetworkPolicyPeer{
							{
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
							},
							{
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"role": "kube-system",
									},
								},
								PodSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      "k8s-app",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"node-local-dns"},
									}},
								},
							},
							// required for node local dns feature, allows egress traffic to CoreDNS
							{
								IPBlock: &networkingv1.IPBlock{
									CIDR: "10.1.0.10/32",
								},
							},
							// required for node local dns feature, allows egress traffic to node local dns cache
							{
								IPBlock: &networkingv1.IPBlock{
									CIDR: "169.254.20.10/32",
								},
							},
						},
						Ports: []networkingv1.NetworkPolicyPort{
							{Protocol: protocolPtr(corev1.ProtocolUDP), Port: intStrPtr(53)},
							{Protocol: protocolPtr(corev1.ProtocolTCP), Port: intStrPtr(53)},
							{Protocol: protocolPtr(corev1.ProtocolUDP), Port: intStrPtr(8053)},
							{Protocol: protocolPtr(corev1.ProtocolTCP), Port: intStrPtr(8053)},
						},
					}},
				}
			},
			inGardenNamespace:       true,
			inIstioSystemNamespace:  true,
			inIstioIngressNamespace: true,
			inShootNamespaces:       true,
		})
	})
})

func protocolPtr(protocol corev1.Protocol) *corev1.Protocol {
	return &protocol
}

func intStrPtr(port int) *intstr.IntOrString {
	v := intstr.FromInt(port)
	return &v
}
