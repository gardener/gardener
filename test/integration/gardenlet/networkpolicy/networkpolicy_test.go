// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	networkpolicyhelper "github.com/gardener/gardener/pkg/controller/networkpolicy/helper"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("NetworkPolicy controller tests", func() {
	var (
		istioSystemNamespace        *corev1.Namespace
		istioIngressNamespace       *corev1.Namespace
		istioExposureClassNamespace *corev1.Namespace
		shootNamespace              *corev1.Namespace
		extensionNamespace          *corev1.Namespace
		fooNamespace                *corev1.Namespace
		customNamespace             *corev1.Namespace
		cluster                     *extensionsv1alpha1.Cluster
	)

	BeforeEach(func() {
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
		istioExposureClassNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "istio-ingress-handler-",
				Labels: map[string]string{
					testID: testRunID,
					v1beta1constants.LabelExposureClassHandlerName: "",
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
		extensionNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "extension-",
				Labels: map[string]string{
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleExtension,
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
		customNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "custom--",
				Labels: map[string]string{
					testID:   testRunID,
					"custom": "namespace",
				},
			},
		}
		cluster = &extensionsv1alpha1.Cluster{
			Spec: extensionsv1alpha1.ClusterSpec{
				CloudProfile: runtime.RawExtension{Raw: []byte("{}")},
				Seed:         runtime.RawExtension{Raw: []byte("{}")},
				Shoot: runtime.RawExtension{Object: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Networking: &gardencorev1beta1.Networking{
							Pods:     ptr.To("10.150.0.0/16"),
							Services: ptr.To("192.168.1.0/17"),
							Nodes:    ptr.To("172.16.2.0/18"),
						},
						Provider: gardencorev1beta1.Provider{
							Workers: []gardencorev1beta1.Worker{},
						},
					},
				}},
			},
		}
	})

	JustBeforeEach(func() {
		By("Create istio-system namespace")
		Expect(testClient.Create(ctx, istioSystemNamespace)).To(Succeed())
		log.Info("Created istio-system namespace for test", "namespaceName", istioSystemNamespace.Name)

		DeferCleanup(func() {
			By("Delete istio-system namespace")
			Expect(testClient.Delete(ctx, istioSystemNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Create istio-ingress namespace")
		Expect(testClient.Create(ctx, istioIngressNamespace)).To(Succeed())
		log.Info("Created istio-ingress namespace for test", "namespaceName", istioIngressNamespace.Name)

		DeferCleanup(func() {
			By("Delete istio-ingress namespace")
			Expect(testClient.Delete(ctx, istioIngressNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Create istio-ingress exposure class namespace")
		Expect(testClient.Create(ctx, istioExposureClassNamespace)).To(Succeed())
		log.Info("Created istio-ingress exposure class namespace for test", "namespaceName", istioExposureClassNamespace.Name)

		DeferCleanup(func() {
			By("Delete istio-ingress exposure class namespace")
			Expect(testClient.Delete(ctx, istioExposureClassNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Create shoot namespace")
		Expect(testClient.Create(ctx, shootNamespace)).To(Succeed())
		log.Info("Created shoot namespace for test", "namespace", client.ObjectKeyFromObject(shootNamespace))

		DeferCleanup(func() {
			By("Delete shoot namespace")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shootNamespace))).To(Succeed())
		})

		By("Create extension namespace")
		Expect(testClient.Create(ctx, extensionNamespace)).To(Succeed())
		log.Info("Created extension namespace for test", "namespace", client.ObjectKeyFromObject(extensionNamespace))

		DeferCleanup(func() {
			By("Delete extension namespace")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, extensionNamespace))).To(Succeed())
		})

		By("Create foo namespace")
		Expect(testClient.Create(ctx, fooNamespace)).To(Succeed())
		log.Info("Created foo namespace for test", "namespace", client.ObjectKeyFromObject(fooNamespace))

		DeferCleanup(func() {
			By("Delete foo namespace")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, fooNamespace))).To(Succeed())
		})

		By("Create custom namespace")
		Expect(testClient.Create(ctx, customNamespace)).To(Succeed())
		log.Info("Created custom namespace for test", "namespace", client.ObjectKeyFromObject(customNamespace))

		DeferCleanup(func() {
			By("Delete custom namespace")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, customNamespace))).To(Succeed())
		})

		By("Create cluster")
		cluster.Name = shootNamespace.Name
		Expect(testClient.Create(ctx, cluster)).To(Succeed())
		log.Info("Created cluster for test", "cluster", client.ObjectKeyFromObject(cluster))

		DeferCleanup(func() {
			By("Delete cluster")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, cluster))).To(Succeed())
		})
	})

	Context("garden namespace", func() {
		It("should have the expected network policies", func() {
			By("Verify that all expected NetworkPolicies get created")
			Eventually(func(g Gomega) []networkingv1.NetworkPolicy {
				networkPolicyList := &networkingv1.NetworkPolicyList{}
				g.Expect(testClient.List(ctx, networkPolicyList, client.InNamespace(gardenNamespace.Name))).To(Succeed())
				return networkPolicyList.Items
			}).Should(ConsistOf(
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("allow-to-runtime-apiserver")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("allow-to-public-networks")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("allow-to-private-networks")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("allow-to-blocked-cidrs")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("allow-to-dns")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("deny-all")})}),
			))
		})
	})

	type testAttributes struct {
		networkPolicyName             string
		expectedNetworkPolicySpec     func(namespaceName string) networkingv1.NetworkPolicySpec
		inGardenNamespace             bool
		inIstioSystemNamespace        bool
		inIstioIngressNamespace       bool
		inIstioExposureClassNamespace bool
		inShootNamespaces             bool
		inExtensionNamespaces         bool
		inCustomNamespace             bool
	}

	defaultTests := func(attrs testAttributes) {
		Context("reconciliation", func() {
			It("should create the network policy in the correct namespaces", func() {
				By("Wait for controller to create the network policy in expected namespaces")
				Eventually(func(g Gomega) {
					if attrs.inShootNamespaces {
						networkPolicy := &networkingv1.NetworkPolicy{}
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: shootNamespace.Name, Name: attrs.networkPolicyName}, networkPolicy)).To(Succeed())
						g.Expect(networkPolicy.Spec).To(Equal(attrs.expectedNetworkPolicySpec(shootNamespace.Name)))
					}

					if attrs.inGardenNamespace {
						networkPolicy := &networkingv1.NetworkPolicy{}
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace.Name, Name: attrs.networkPolicyName}, networkPolicy)).To(Succeed())
						g.Expect(networkPolicy.Spec).To(Equal(attrs.expectedNetworkPolicySpec(gardenNamespace.Name)))
					}

					if attrs.inIstioSystemNamespace {
						networkPolicy := &networkingv1.NetworkPolicy{}
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: istioSystemNamespace.Name, Name: attrs.networkPolicyName}, networkPolicy)).To(Succeed())
						g.Expect(networkPolicy.Spec).To(Equal(attrs.expectedNetworkPolicySpec(istioSystemNamespace.Name)))
					}

					if attrs.inIstioIngressNamespace {
						networkPolicy := &networkingv1.NetworkPolicy{}
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: istioIngressNamespace.Name, Name: attrs.networkPolicyName}, networkPolicy)).To(Succeed())
						g.Expect(networkPolicy.Spec).To(Equal(attrs.expectedNetworkPolicySpec(istioIngressNamespace.Name)))
					}

					if attrs.inIstioExposureClassNamespace {
						networkPolicy := &networkingv1.NetworkPolicy{}
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: istioExposureClassNamespace.Name, Name: attrs.networkPolicyName}, networkPolicy)).To(Succeed())
						g.Expect(networkPolicy.Spec).To(Equal(attrs.expectedNetworkPolicySpec(istioExposureClassNamespace.Name)))
					}

					if attrs.inExtensionNamespaces {
						networkPolicy := &networkingv1.NetworkPolicy{}
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: extensionNamespace.Name, Name: attrs.networkPolicyName}, networkPolicy)).To(Succeed())
						g.Expect(networkPolicy.Spec).To(Equal(attrs.expectedNetworkPolicySpec(extensionNamespace.Name)))
					}

					if attrs.inCustomNamespace {
						networkPolicy := &networkingv1.NetworkPolicy{}
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: customNamespace.Name, Name: attrs.networkPolicyName}, networkPolicy)).To(Succeed())
						g.Expect(networkPolicy.Spec).To(Equal(attrs.expectedNetworkPolicySpec(customNamespace.Name)))
					}
				}).Should(Succeed())

				By("Ensure controller does not create the network policy in unexpected namespaces")
				Consistently(func(g Gomega) {
					if !attrs.inShootNamespaces {
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: shootNamespace.Name, Name: attrs.networkPolicyName}, &networkingv1.NetworkPolicy{})).Should(BeNotFoundError())
					}

					if !attrs.inGardenNamespace {
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace.Name, Name: attrs.networkPolicyName}, &networkingv1.NetworkPolicy{})).Should(BeNotFoundError())
					}

					if !attrs.inIstioSystemNamespace {
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: istioSystemNamespace.Name, Name: attrs.networkPolicyName}, &networkingv1.NetworkPolicy{})).Should(BeNotFoundError())
					}

					if !attrs.inIstioIngressNamespace {
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: istioIngressNamespace.Name, Name: attrs.networkPolicyName}, &networkingv1.NetworkPolicy{})).Should(BeNotFoundError())
					}

					if !attrs.inIstioExposureClassNamespace {
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: istioExposureClassNamespace.Name, Name: attrs.networkPolicyName}, &networkingv1.NetworkPolicy{})).Should(BeNotFoundError())
					}

					if !attrs.inExtensionNamespaces {
						g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: extensionNamespace.Name, Name: attrs.networkPolicyName}, &networkingv1.NetworkPolicy{})).Should(BeNotFoundError())
					}

					g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: fooNamespace.Name, Name: attrs.networkPolicyName}, &networkingv1.NetworkPolicy{})).Should(BeNotFoundError())
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
				}

				By("Create network policy")
				Expect(testClient.Create(ctx, networkPolicy)).To(Succeed())

				DeferCleanup(func() {
					By("Delete network policy")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, networkPolicy))).To(Succeed())
				})

				By("Wait for controller to delete the network policy")
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)
				}).Should(BeNotFoundError())
			})
		})
	}

	Describe("deny-all", func() {
		defaultTests(testAttributes{
			networkPolicyName: "deny-all",
			expectedNetworkPolicySpec: func(string) networkingv1.NetworkPolicySpec {
				return networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{},
					PolicyTypes: []networkingv1.PolicyType{"Ingress", "Egress"},
				}
			},
			inGardenNamespace:             true,
			inIstioSystemNamespace:        true,
			inIstioIngressNamespace:       true,
			inIstioExposureClassNamespace: true,
			inShootNamespaces:             true,
			inExtensionNamespaces:         true,
			inCustomNamespace:             true,
		})
	})

	Describe("allow-to-runtime-apiserver", func() {
		var expectedNetworkPolicySpec networkingv1.NetworkPolicySpec

		JustBeforeEach(func() {
			kubernetesEndpoint := &corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "kubernetes"}}
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(kubernetesEndpoint), kubernetesEndpoint)).To(Succeed())

			egressRules, err := networkpolicyhelper.GetEgressRules(kubernetesEndpoint.Subsets...)
			Expect(err).ToNot(HaveOccurred())

			expectedNetworkPolicySpec = networkingv1.NetworkPolicySpec{
				Egress:      egressRules,
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"networking.gardener.cloud/to-runtime-apiserver": "allowed"}},
				PolicyTypes: []networkingv1.PolicyType{"Egress"},
			}
		})

		defaultTests(testAttributes{
			networkPolicyName:         "allow-to-runtime-apiserver",
			expectedNetworkPolicySpec: func(string) networkingv1.NetworkPolicySpec { return expectedNetworkPolicySpec },
			inGardenNamespace:         true,
			inIstioSystemNamespace:    true,
			inIstioIngressNamespace:   true,
			inShootNamespaces:         true,
			inExtensionNamespaces:     true,
			inCustomNamespace:         true,
		})
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
					}, {
						IPBlock: &networkingv1.IPBlock{
							CIDR: "::/0",
							Except: []string{
								"fe80::/10",
								"fc00::/7",
							},
						},
					}},
				}},
				PolicyTypes: []networkingv1.PolicyType{"Egress"},
			}
		)

		defaultTests(testAttributes{
			networkPolicyName:         networkPolicyName,
			expectedNetworkPolicySpec: func(string) networkingv1.NetworkPolicySpec { return expectedNetworkPolicySpec },
			inGardenNamespace:         true,
			inShootNamespaces:         true,
			inExtensionNamespaces:     true,
			inCustomNamespace:         true,
		})
	})

	Describe("allow-to-private-networks", func() {
		defaultTests(testAttributes{
			networkPolicyName: "allow-to-private-networks",
			expectedNetworkPolicySpec: func(namespaceName string) networkingv1.NetworkPolicySpec {
				out := networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed}},
					PolicyTypes: []networkingv1.PolicyType{"Egress"},
					Egress: []networkingv1.NetworkPolicyEgressRule{{
						To: []networkingv1.NetworkPolicyPeer{
							{IPBlock: &networkingv1.IPBlock{CIDR: "10.0.0.0/8", Except: []string{"10.0.0.0/16", "10.1.0.0/16", "10.2.0.0/16"}}},
							{IPBlock: &networkingv1.IPBlock{CIDR: "172.16.0.0/12"}},
							{IPBlock: &networkingv1.IPBlock{CIDR: "192.168.0.0/16"}},
							{IPBlock: &networkingv1.IPBlock{CIDR: "100.64.0.0/10"}},
						},
					}, {
						To: []networkingv1.NetworkPolicyPeer{
							{IPBlock: &networkingv1.IPBlock{CIDR: "fe80::/10"}},
							{IPBlock: &networkingv1.IPBlock{CIDR: "fc00::/7"}},
						},
					}},
				}

				if strings.HasPrefix(namespaceName, "shoot--") {
					out.Egress[0].To[0].IPBlock.Except = append(out.Egress[0].To[0].IPBlock.Except, "10.150.0.0/16")
					out.Egress[0].To[1].IPBlock.Except = append(out.Egress[0].To[1].IPBlock.Except, "172.16.2.0/18")
					out.Egress[0].To[2].IPBlock.Except = append(out.Egress[0].To[2].IPBlock.Except, "192.168.1.0/17")
				}

				return out
			},
			inGardenNamespace:     true,
			inShootNamespaces:     true,
			inExtensionNamespaces: true,
			inCustomNamespace:     true,
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
			expectedNetworkPolicySpec: func(string) networkingv1.NetworkPolicySpec { return expectedNetworkPolicySpec },
			inGardenNamespace:         true,
			inShootNamespaces:         true,
			inExtensionNamespaces:     true,
			inCustomNamespace:         true,
		})
	})

	Describe("allow-to-dns", func() {
		defaultTests(testAttributes{
			networkPolicyName: "allow-to-dns",
			expectedNetworkPolicySpec: func(string) networkingv1.NetworkPolicySpec {
				return networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyToDNS: v1beta1constants.LabelNetworkPolicyAllowed}},
					PolicyTypes: []networkingv1.PolicyType{"Egress"},
					Egress: []networkingv1.NetworkPolicyEgressRule{{
						To: []networkingv1.NetworkPolicyPeer{
							{
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"kubernetes.io/metadata.name": "kube-system",
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
										"kubernetes.io/metadata.name": "kube-system",
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
							// required for node local dns feature, allows egress traffic to node local dns cache
							{
								IPBlock: &networkingv1.IPBlock{
									CIDR: "169.254.20.10/32",
								},
							},
							// required for node local dns feature, allows egress traffic to node local dns cache
							{
								IPBlock: &networkingv1.IPBlock{
									CIDR: "fd30:1319:f1e:230b::1/128",
								},
							},
							// required for node local dns feature, allows egress traffic to CoreDNS
							{
								IPBlock: &networkingv1.IPBlock{
									CIDR: "10.1.0.10/32",
								},
							},
						},
						Ports: []networkingv1.NetworkPolicyPort{
							{Protocol: ptr.To(corev1.ProtocolUDP), Port: ptr.To(intstr.FromInt32(53))},
							{Protocol: ptr.To(corev1.ProtocolTCP), Port: ptr.To(intstr.FromInt32(53))},
							{Protocol: ptr.To(corev1.ProtocolUDP), Port: ptr.To(intstr.FromInt32(8053))},
							{Protocol: ptr.To(corev1.ProtocolTCP), Port: ptr.To(intstr.FromInt32(8053))},
						},
					}},
				}
			},
			inGardenNamespace:             true,
			inIstioSystemNamespace:        true,
			inIstioIngressNamespace:       true,
			inIstioExposureClassNamespace: true,
			inShootNamespaces:             true,
			inExtensionNamespaces:         true,
			inCustomNamespace:             true,
		})
	})

	Context("updates", func() {
		networkPolicyName := "allow-to-dns"

		It("should reconcile the network policy when it is changed by a third party", func() {
			By("Fetch network policy")
			networkPolicy := &networkingv1.NetworkPolicy{}
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace.Name, Name: networkPolicyName}, networkPolicy)
			}).Should(Succeed())
			desiredSpec := networkPolicy.Spec

			By("Modify network policy to trigger reconciliation")
			networkPolicy.Spec.PodSelector.MatchLabels = map[string]string{"foo": "bar"}
			Expect(testClient.Update(ctx, networkPolicy)).To(Succeed())

			By("Wait for controller to reconcile the network policy")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace.Name, Name: networkPolicyName}, networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(desiredSpec))
		})

		It("should not update the network policy if nothing changed", func() {
			By("Fetch network policy")
			beforeShootNetworkPolicy := &networkingv1.NetworkPolicy{}
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace.Name, Name: networkPolicyName}, beforeShootNetworkPolicy)
			}).Should(Succeed())

			By("Modify namespace to trigger reconciliation")
			gardenNamespace.Labels["foo"] = "bar"
			Expect(testClient.Update(ctx, gardenNamespace)).To(Succeed())

			By("Ensure controller does not reconcile the network policy unnecessarily")
			Consistently(func(g Gomega) string {
				networkPolicy := &networkingv1.NetworkPolicy{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace.Name, Name: networkPolicyName}, networkPolicy)).To(Succeed())
				return networkPolicy.ResourceVersion
			}).Should(Equal(beforeShootNetworkPolicy.ResourceVersion))
		})
	})

	Context("seed gets registered as garden", func() {
		var garden *operatorv1alpha1.Garden

		BeforeEach(func() {
			garden = &operatorv1alpha1.Garden{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "garden-",
					Labels: map[string]string{
						testID: testRunID,
					},
				},
				Spec: operatorv1alpha1.GardenSpec{
					RuntimeCluster: operatorv1alpha1.RuntimeCluster{
						Networking: operatorv1alpha1.RuntimeNetworking{
							Pods:     []string{"10.1.0.0/16"},
							Services: []string{"10.2.0.0/16"},
						},
						Ingress: operatorv1alpha1.Ingress{
							Domains: []operatorv1alpha1.DNSDomain{{Name: "ingress.dev.seed.example.com"}},
							Controller: gardencorev1beta1.IngressController{
								Kind: "nginx",
							},
						},
					},
					VirtualCluster: operatorv1alpha1.VirtualCluster{
						DNS: operatorv1alpha1.DNS{
							Domains: []operatorv1alpha1.DNSDomain{{Name: "virtual-garden.local.gardener.cloud"}},
						},
						Gardener: operatorv1alpha1.Gardener{
							ClusterIdentity: "test",
						},
						Kubernetes: operatorv1alpha1.Kubernetes{
							Version: "1.31.1",
						},
						Maintenance: operatorv1alpha1.Maintenance{
							TimeWindow: gardencorev1beta1.MaintenanceTimeWindow{
								Begin: "220000+0100",
								End:   "230000+0100",
							},
						},
						Networking: operatorv1alpha1.Networking{
							Services: []string{"100.64.0.0/13"},
						},
					},
				},
			}
		})

		It("should not call the cancel function", func() {
			Consistently(func() <-chan struct{} {
				return testContext.Done()
			}).ShouldNot(BeClosed())
		})

		It("should call the cancel function", func() {
			By("Create Garden")
			Expect(testClient.Create(ctx, garden)).To(Succeed())
			log.Info("Created Garden for test", "garden", garden.Name)

			DeferCleanup(func() {
				By("Delete Garden")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, garden))).To(Succeed())
			})

			Eventually(func() <-chan struct{} {
				return testContext.Done()
			}).Should(BeClosed())
		})
	})
})
