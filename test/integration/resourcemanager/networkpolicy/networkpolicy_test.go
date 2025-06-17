// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy_test

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("NetworkPolicy Controller tests", func() {
	var (
		namespace      *corev1.Namespace
		otherNamespace *corev1.Namespace
		service        *corev1.Service

		serviceSelector         = map[string]string{"foo": "bar"}
		customPodLabelSelector1 = "custom-selector1"
		customPodLabelSelector2 = "custom-selector2"

		port1Protocol          = corev1.ProtocolTCP
		port1ServicePort int32 = 1234
		port1TargetPort        = intstr.FromInt32(5678)
		port1Suffix            = fmt.Sprintf("-%s-%s", strings.ToLower(string(port1Protocol)), port1TargetPort.String())

		port2Protocol          = corev1.ProtocolUDP
		port2ServicePort int32 = 9012
		port2TargetPort        = intstr.FromString("testport")
		port2Suffix            = fmt.Sprintf("-%s-%s", strings.ToLower(string(port2Protocol)), port2TargetPort.String())

		port3Protocol   = corev1.ProtocolUDP
		port3TargetPort = intstr.FromString("testport2")
		port3Suffix     = fmt.Sprintf("-%s-%s", strings.ToLower(string(port3Protocol)), port3TargetPort.String())

		port4Protocol   = corev1.ProtocolTCP
		port4TargetPort = intstr.FromInt32(3456)
		port4Suffix     = fmt.Sprintf("-%s-%s", strings.ToLower(string(port4Protocol)), port4TargetPort.String())

		port5Protocol   = corev1.ProtocolUDP
		port5TargetPort = intstr.FromInt32(7891)
		port5Suffix     = fmt.Sprintf("-%s-%s", strings.ToLower(string(port5Protocol)), port5TargetPort.String())

		port6Protocol   = corev1.ProtocolTCP
		port6TargetPort = intstr.FromInt32(1023)
		port6Suffix     = fmt.Sprintf("-%s-%s", strings.ToLower(string(port6Protocol)), port6TargetPort.String())

		ensureNetworkPolicies = func(asyncAssertion func(int, any, ...any) AsyncAssertion, should bool) func() {
			return func() {
				assertedFunc := func(g Gomega) []string {
					networkPolicyList := &networkingv1.NetworkPolicyList{}
					g.Expect(testClient.List(ctx, networkPolicyList, client.InNamespace(service.Namespace))).To(Succeed())
					return test.ObjectNames(networkPolicyList)
				}
				expectation := []string{
					"ingress-to-" + service.Name + port1Suffix,
					"egress-to-" + service.Name + port1Suffix,
					"ingress-to-" + service.Name + port2Suffix,
					"egress-to-" + service.Name + port2Suffix,
				}

				if should {
					asyncAssertion(1, assertedFunc).Should(ContainElements(expectation))
				} else {
					// TODO(tobschli): Change this to the `ShouldNot(ContainAnyOf())` matcher, once https://github.com/gardener/gardener/pull/12317 is merged.
					asyncAssertion(1, assertedFunc).ShouldNot(ContainElements(expectation))
				}
			}
		}
		ensureNetworkPoliciesGetCreated      = ensureNetworkPolicies(EventuallyWithOffset, true)
		ensureNetworkPoliciesGetDeleted      = ensureNetworkPolicies(EventuallyWithOffset, false)
		ensureNetworkPoliciesDoNotGetCreated = ensureNetworkPolicies(ConsistentlyWithOffset, false)
		ensureNetworkPoliciesDoNotGetDeleted = ensureNetworkPolicies(ConsistentlyWithOffset, true)

		ensureCrossNamespaceNetworkPolicies = func(asyncAssertion func(int, any, ...any) AsyncAssertion, should bool) func() {
			return func() {
				// ingress rules
				assertedFunc := func(g Gomega) []string {
					networkPolicyList := &networkingv1.NetworkPolicyList{}
					g.Expect(testClient.List(ctx, networkPolicyList, client.InNamespace(service.Namespace))).To(Succeed())
					return test.ObjectNames(networkPolicyList)
				}
				expectation := []string{
					"ingress-to-" + service.Name + port1Suffix + "-from-" + otherNamespace.Name,
					"ingress-to-" + service.Name + port2Suffix + "-from-" + otherNamespace.Name,
				}

				if should {
					asyncAssertion(1, assertedFunc).Should(ContainElements(expectation))
				} else {
					// TODO(tobschli): Change this to the `ShouldNot(ContainAnyOf())` matcher, once https://github.com/gardener/gardener/pull/12317 is merged.
					asyncAssertion(1, assertedFunc).ShouldNot(ContainElements(expectation))
				}

				// egress rules
				assertedFunc = func(g Gomega) []string {
					networkPolicyList := &networkingv1.NetworkPolicyList{}
					g.Expect(testClient.List(ctx, networkPolicyList, client.InNamespace(otherNamespace.Name))).To(Succeed())
					return test.ObjectNames(networkPolicyList)
				}
				expectation = []string{
					"egress-to-" + service.Namespace + "-" + service.Name + port1Suffix,
					"egress-to-" + service.Namespace + "-" + service.Name + port2Suffix,
				}

				if should {
					asyncAssertion(1, assertedFunc).Should(ContainElements(expectation))
				} else {
					// TODO(tobschli): Change this to the `ShouldNot(ContainAnyOf())` matcher, once https://github.com/gardener/gardener/pull/12317 is merged.
					asyncAssertion(1, assertedFunc).ShouldNot(ContainElements(expectation))
				}
			}
		}
		ensureCrossNamespaceNetworkPoliciesGetCreated      = ensureCrossNamespaceNetworkPolicies(EventuallyWithOffset, true)
		ensureCrossNamespaceNetworkPoliciesGetDeleted      = ensureCrossNamespaceNetworkPolicies(EventuallyWithOffset, false)
		ensureCrossNamespaceNetworkPoliciesDoNotGetCreated = ensureCrossNamespaceNetworkPolicies(ConsistentlyWithOffset, false)

		ensureNetworkPoliciesWithCustomPodLabelSelectors = func(asyncAssertion func(int, any, ...any) AsyncAssertion, should bool) func() {
			return func() {
				assertedFunc := func(g Gomega) []string {
					networkPolicyList := &networkingv1.NetworkPolicyList{}
					g.Expect(testClient.List(ctx, networkPolicyList, client.InNamespace(service.Namespace))).To(Succeed())
					return test.ObjectNames(networkPolicyList)
				}
				expectation := []string{
					"ingress-to-" + service.Name + port3Suffix + "-via-" + customPodLabelSelector1,
					"egress-to-" + service.Name + port3Suffix + "-via-" + customPodLabelSelector1,
					"ingress-to-" + service.Name + port4Suffix + "-via-" + customPodLabelSelector1,
					"egress-to-" + service.Name + port4Suffix + "-via-" + customPodLabelSelector1,
					"ingress-to-" + service.Name + port5Suffix + "-via-" + customPodLabelSelector2,
					"egress-to-" + service.Name + port5Suffix + "-via-" + customPodLabelSelector2,
					"ingress-to-" + service.Name + port6Suffix + "-via-" + customPodLabelSelector2,
					"egress-to-" + service.Name + port6Suffix + "-via-" + customPodLabelSelector2,
				}

				if should {
					asyncAssertion(1, assertedFunc).Should(ContainElements(expectation))
				} else {
					// TODO(tobschli): Change this to the `ShouldNot(ContainAnyOf())` matcher, once https://github.com/gardener/gardener/pull/12317 is merged.
					asyncAssertion(1, assertedFunc).ShouldNot(ContainElements(expectation))
				}
			}
		}
		ensureNetworkPoliciesWithCustomPodLabelSelectorsGetCreated = ensureNetworkPoliciesWithCustomPodLabelSelectors(EventuallyWithOffset, true)
		ensureNetworkPoliciesWithCustomPodLabelSelectorsGetDeleted = ensureNetworkPoliciesWithCustomPodLabelSelectors(EventuallyWithOffset, false)

		ensureIngressFromWorldNetworkPolicy = func(asyncAssertion func(int, any, ...any) AsyncAssertion, should bool) func() {
			return func() {
				assertedFunc := func(g Gomega) []string {
					networkPolicyList := &networkingv1.NetworkPolicyList{}
					g.Expect(testClient.List(ctx, networkPolicyList, client.InNamespace(service.Namespace))).To(Succeed())
					return test.ObjectNames(networkPolicyList)
				}
				expectation := []string{
					"ingress-to-" + service.Name + "-from-world",
				}

				if should {
					asyncAssertion(1, assertedFunc).Should(ContainElements(expectation))
				} else {
					// TODO(tobschli): Change this to the `ShouldNot(ContainAnyOf())` matcher, once https://github.com/gardener/gardener/pull/12317 is merged.
					asyncAssertion(1, assertedFunc).ShouldNot(ContainElements(expectation))
				}
			}
		}
		ensureIngressFromWorldNetworkPolicyGetsCreated       = ensureIngressFromWorldNetworkPolicy(EventuallyWithOffset, true)
		ensureIngressFromWorldNetworkPolicyGetsDeleted       = ensureIngressFromWorldNetworkPolicy(EventuallyWithOffset, false)
		ensureIngressFromWorldNetworkPolicyDoesNotGetCreated = ensureIngressFromWorldNetworkPolicy(ConsistentlyWithOffset, false)
	)

	BeforeEach(func() {
		logBuffer.Reset()

		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test-ns-" + testRunID,
				Labels: map[string]string{testID: testRunID},
			},
		}
		otherNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "other-ns-" + testRunID,
				Labels: map[string]string{"other": "namespace"},
			},
		}
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "service-",
				Namespace:    namespace.Name,
			},
			Spec: corev1.ServiceSpec{
				Selector: serviceSelector,
				Ports: []corev1.ServicePort{
					{Name: "port1", Port: port1ServicePort, Protocol: port1Protocol, TargetPort: port1TargetPort},
					{Name: "port2", Port: port2ServicePort, Protocol: port2Protocol, TargetPort: port2TargetPort},
				},
			},
		}
	})

	JustBeforeEach(func() {
		By("Create test Namespace")
		Expect(testClient.Create(ctx, namespace)).To(Succeed())
		log.Info("Created test Namespace", "namespace", client.ObjectKeyFromObject(namespace))

		DeferCleanup(func() {
			By("Delete test Namespace")
			Expect(testClient.Delete(ctx, namespace)).To(Or(Succeed(), BeNotFoundError()))
			log.Info("Deleted test Namespace", "namespace", client.ObjectKeyFromObject(namespace))

			By("Wait until manager has observed test Namespace deletion")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(namespace), namespace)
			}).Should(BeNotFoundError())
		})

		By("Create other Namespace")
		Expect(testClient.Create(ctx, otherNamespace)).To(Succeed())
		log.Info("Created other Namespace", "namespace", client.ObjectKeyFromObject(otherNamespace))

		DeferCleanup(func() {
			By("Delete other Namespace")
			Expect(testClient.Delete(ctx, otherNamespace)).To(Or(Succeed(), BeNotFoundError()))
			log.Info("Deleted other Namespace", "namespace", client.ObjectKeyFromObject(otherNamespace))

			By("Wait until manager has observed other Namespace deletion")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(otherNamespace), otherNamespace)
			}).Should(BeNotFoundError())
		})

		By("Create Service")
		Expect(testClient.Create(ctx, service)).To(Succeed())
		log.Info("Created Service", "service", client.ObjectKeyFromObject(service))

		DeferCleanup(func() {
			By("Delete Service")
			Expect(testClient.Delete(ctx, service)).To(Or(Succeed(), BeNotFoundError()))
			log.Info("Deleted Service", "service", client.ObjectKeyFromObject(service))

			By("Wait until manager has observed Service deletion")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(service), service)
			}).Should(BeNotFoundError())
		})
	})

	Context("without pod selector in service", func() {
		BeforeEach(func() {
			service.Spec.Selector = nil
		})

		It("should not create any network policies", func() {
			By("Ensure no policies are created")
			ensureNetworkPoliciesDoNotGetCreated()
		})
	})

	Context("service in handled namespace", func() {
		It("should create the expected network policies", func() {
			By("Wait until ingress policy was created for first port")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "ingress-to-" + service.Name + port1Suffix, Namespace: service.Namespace}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
				PodSelector: metav1.LabelSelector{MatchLabels: serviceSelector},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{
					From:  []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + service.Name + port1Suffix: "allowed"}}}},
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &port1Protocol, Port: &port1TargetPort}},
				}},
			}))

			By("Wait until egress policy was created for first port")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "egress-to-" + service.Name + port1Suffix, Namespace: service.Namespace}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + service.Name + port1Suffix: "allowed"}},
				Egress: []networkingv1.NetworkPolicyEgressRule{{
					To:    []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: serviceSelector}}},
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &port1Protocol, Port: &port1TargetPort}},
				}},
			}))

			By("Wait until ingress policy was created for second port")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "ingress-to-" + service.Name + port2Suffix, Namespace: service.Namespace}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
				PodSelector: metav1.LabelSelector{MatchLabels: serviceSelector},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{
					From:  []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + service.Name + port2Suffix: "allowed"}}}},
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &port2Protocol, Port: &port2TargetPort}},
				}},
			}))

			By("Wait until egress policy was created for second port")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "egress-to-" + service.Name + port2Suffix, Namespace: service.Namespace}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + service.Name + port2Suffix: "allowed"}},
				Egress: []networkingv1.NetworkPolicyEgressRule{{
					To:    []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: serviceSelector}}},
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &port2Protocol, Port: &port2TargetPort}},
				}},
			}))
		})

		It("should not create any cross-namespace policies or ingress-from-world policy", func() {
			ensureCrossNamespaceNetworkPoliciesDoNotGetCreated()
			ensureIngressFromWorldNetworkPolicyDoesNotGetCreated()
		})

		It("should reconcile the policies when the ports in service are changed", func() {
			By("Wait until all policies are created")
			ensureNetworkPoliciesGetCreated()

			By("Patch Service")
			patch := client.MergeFrom(service.DeepCopy())
			service.Spec.Ports = []corev1.ServicePort{service.Spec.Ports[1]}
			service.Spec.Ports = append(service.Spec.Ports, corev1.ServicePort{Name: "newport", Port: 1357, Protocol: corev1.ProtocolUDP, TargetPort: intstr.FromInt32(2468)})
			Expect(testClient.Patch(ctx, service, patch)).To(Succeed())

			By("Wait until all policies were reconciled")
			Eventually(func(g Gomega) []string {
				networkPolicyList := &networkingv1.NetworkPolicyList{}
				g.Expect(testClient.List(ctx, networkPolicyList, client.InNamespace(service.Namespace))).To(Succeed())
				return test.ObjectNames(networkPolicyList)
			}).Should(And(
				// TODO(tobschli): Change this to the `ShouldNot(ContainAnyOf())` matcher, once https://github.com/gardener/gardener/pull/12317 is merged.
				Not(ContainElements(
					"ingress-to-"+service.Name+port1Suffix,
					"egress-to-"+service.Name+port1Suffix,
				)),
				ContainElements(
					"ingress-to-"+service.Name+port2Suffix,
					"egress-to-"+service.Name+port2Suffix,
					"ingress-to-"+service.Name+"-udp-2468",
					"egress-to-"+service.Name+"-udp-2468",
				),
			))
		})

		It("should delete the policies when the pod selector in service is removed", func() {
			By("Wait until all policies are created")
			ensureNetworkPoliciesGetCreated()

			By("Patch Service")
			patch := client.MergeFrom(service.DeepCopy())
			service.Spec.Selector = nil
			Expect(testClient.Patch(ctx, service, patch)).To(Succeed())

			By("Wait until all policies are deleted")
			ensureNetworkPoliciesGetDeleted()
		})

		It("should delete the policies when the service gets deleted", func() {
			By("Wait until all policies are created")
			ensureNetworkPoliciesGetCreated()

			By("Delete Service")
			Expect(testClient.Delete(ctx, service)).To(Succeed())

			By("Wait until all policies are deleted")
			ensureNetworkPoliciesGetDeleted()
		})

		It("should delete the policies when the namespace is no longer handled", func() {
			By("Wait until all policies are created")
			ensureNetworkPoliciesGetCreated()

			By("Patch Namespace and remove label")
			patch := client.MergeFrom(namespace.DeepCopy())
			namespace.Labels[testID] = "foo"
			Expect(testClient.Patch(ctx, namespace, patch)).To(Succeed())

			By("Wait until all policies are deleted")
			ensureNetworkPoliciesGetDeleted()
		})

		It("should clean up the policies when the service is already gone", func() {
			networkPolicy1 := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "policy1",
					Namespace: namespace.Name,
					Labels: map[string]string{
						"networking.resources.gardener.cloud/service-name":      "foo",
						"networking.resources.gardener.cloud/service-namespace": namespace.Name,
					},
				},
			}
			Expect(testClient.Create(ctx, networkPolicy1)).To(Succeed())

			networkPolicy2 := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "policy2",
					Namespace: namespace.Name,
					Labels: map[string]string{
						"networking.resources.gardener.cloud/service-name":      "foo",
						"networking.resources.gardener.cloud/service-namespace": "bar",
					},
				},
			}
			Expect(testClient.Create(ctx, networkPolicy2)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy1), networkPolicy1)).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy2), networkPolicy1)).To(BeNotFoundError())
			}).Should(Succeed())
		})

		When("label selector exceeds maximum length of 63 characters for labels", func() {
			BeforeEach(func() {
				service.Name = "this-is-a-very-long-svc-name-which-will-exceed-max-length"
			})

			It("should shorten the label selector key", func() {
				By("Ensure expected policies are created with shortened label selector key")
				Eventually(func(g Gomega) {
					ingressPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "ingress-to-" + service.Name + port1Suffix, Namespace: service.Namespace}}
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(ingressPolicy), ingressPolicy)).To(Succeed())
					g.Expect(ingressPolicy.Spec.Ingress[0].From[0].PodSelector.MatchLabels).To(Equal(map[string]string{"networking.resources.gardener.cloud/to-this-is-a-very-long-svc-name-which-will-exceed-max-len-7c268": "allowed"}))

					egressPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "egress-to-" + service.Name + port1Suffix, Namespace: service.Namespace}}
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(egressPolicy), egressPolicy)).To(Succeed())
					g.Expect(egressPolicy.Spec.PodSelector.MatchLabels).To(Equal(map[string]string{"networking.resources.gardener.cloud/to-this-is-a-very-long-svc-name-which-will-exceed-max-len-7c268": "allowed"}))
				}).Should(Succeed())

				By("Ensure controller prints information about mutated pod label selector")
				Eventually(func() string { return logBuffer.String() }).Should(ContainSubstring("Usual pod label selector contained at least one key exceeding 63 characters - it had to be mutated"))
			})
		})
	})

	Context("service in non-handled namespace", func() {
		BeforeEach(func() {
			service.Namespace = otherNamespace.Name
		})

		It("should not create any network policies", func() {
			By("Ensure no policies are created")
			ensureNetworkPoliciesDoNotGetCreated()
		})

		It("should create network policies as soon as the namespace is handled", func() {
			By("Patch Namespace")
			patch := client.MergeFrom(otherNamespace.DeepCopy())
			metav1.SetMetaDataLabel(&otherNamespace.ObjectMeta, testID, testRunID)
			Expect(testClient.Patch(ctx, otherNamespace, patch)).To(Succeed())

			By("Ensure no policies are created")
			ensureNetworkPoliciesGetCreated()
		})
	})

	Context("service with namespace selector", func() {
		BeforeEach(func() {
			metav1.SetMetaDataAnnotation(&service.ObjectMeta, "networking.resources.gardener.cloud/namespace-selectors", `[{"matchLabels":{"other":"namespace"}}]`)
		})

		It("should create the expected cross-namespace network policies", func() {
			ensureNetworkPoliciesGetCreated()

			By("Wait until ingress from other-namespace policy was created for first port")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "ingress-to-" + service.Name + port1Suffix + "-from-" + otherNamespace.Name, Namespace: service.Namespace}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
				PodSelector: metav1.LabelSelector{MatchLabels: serviceSelector},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{
					From: []networkingv1.NetworkPolicyPeer{{
						PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + service.Namespace + "-" + service.Name + port1Suffix: "allowed"}},
						NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": otherNamespace.Name}},
					}},
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &port1Protocol, Port: &port1TargetPort}},
				}},
			}))

			By("Wait until egress from other-namespace policy was created for first port")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "egress-to-" + service.Namespace + "-" + service.Name + port1Suffix, Namespace: otherNamespace.Name}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + service.Namespace + "-" + service.Name + port1Suffix: "allowed"}},
				Egress: []networkingv1.NetworkPolicyEgressRule{{
					To: []networkingv1.NetworkPolicyPeer{{
						PodSelector:       &metav1.LabelSelector{MatchLabels: serviceSelector},
						NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": service.Namespace}},
					}},
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &port1Protocol, Port: &port1TargetPort}},
				}},
			}))

			By("Wait until ingress from other-namespace policy was created for second port")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "ingress-to-" + service.Name + port2Suffix + "-from-" + otherNamespace.Name, Namespace: service.Namespace}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
				PodSelector: metav1.LabelSelector{MatchLabels: serviceSelector},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{
					From: []networkingv1.NetworkPolicyPeer{{
						PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + service.Namespace + "-" + service.Name + port2Suffix: "allowed"}},
						NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": otherNamespace.Name}},
					}},
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &port2Protocol, Port: &port2TargetPort}},
				}},
			}))

			By("Wait until egress from other-namespace policy was created for second port")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "egress-to-" + service.Namespace + "-" + service.Name + port2Suffix, Namespace: otherNamespace.Name}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + service.Namespace + "-" + service.Name + port2Suffix: "allowed"}},
				Egress: []networkingv1.NetworkPolicyEgressRule{{
					To: []networkingv1.NetworkPolicyPeer{{
						PodSelector:       &metav1.LabelSelector{MatchLabels: serviceSelector},
						NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": service.Namespace}},
					}},
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &port2Protocol, Port: &port2TargetPort}},
				}},
			}))
		})

		It("should reconcile the policies when the ports in service are changed", func() {
			By("Wait until all policies are created")
			ensureNetworkPoliciesGetCreated()
			ensureCrossNamespaceNetworkPoliciesGetCreated()

			By("Patch Service")
			patch := client.MergeFrom(service.DeepCopy())
			service.Spec.Ports = []corev1.ServicePort{service.Spec.Ports[1]}
			service.Spec.Ports = append(service.Spec.Ports, corev1.ServicePort{Name: "newport", Port: 1357, Protocol: corev1.ProtocolUDP, TargetPort: intstr.FromInt32(2468)})
			Expect(testClient.Patch(ctx, service, patch)).To(Succeed())

			By("Wait until cross-namespace policies were reconciled")
			Eventually(func(g Gomega) []networkingv1.NetworkPolicy {
				networkPolicyList := &networkingv1.NetworkPolicyList{}
				g.Expect(testClient.List(ctx, networkPolicyList, client.InNamespace(service.Namespace))).To(Succeed())
				return networkPolicyList.Items
			}).Should(And(
				Not(ContainElements(
					MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("ingress-to-" + service.Name + port1Suffix + "-from-" + otherNamespace.Name)})}),
				)),
				ContainElements(
					MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("ingress-to-" + service.Name + port2Suffix + "-from-" + otherNamespace.Name)})}),
					MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("ingress-to-" + service.Name + "-udp-2468-from-" + otherNamespace.Name)})}),
				),
			))

			Eventually(func(g Gomega) []string {
				networkPolicyList := &networkingv1.NetworkPolicyList{}
				g.Expect(testClient.List(ctx, networkPolicyList, client.InNamespace(otherNamespace.Name))).To(Succeed())
				return test.ObjectNames(networkPolicyList)
			}).Should(And(
				// TODO(tobschli): Change this to the `ShouldNot(ContainAnyOf())` matcher, once https://github.com/gardener/gardener/pull/12317 is merged.
				Not(ContainElements(
					"egress-to-"+service.Namespace+"-"+service.Name+port1Suffix,
				)),
				ContainElements(
					"egress-to-"+service.Namespace+"-"+service.Name+port2Suffix,
					"egress-to-"+service.Namespace+"-"+service.Name+"-udp-2468",
				),
			))
		})

		It("should delete the policies when the pod selector in service is removed", func() {
			By("Wait until all policies are created")
			ensureNetworkPoliciesGetCreated()
			ensureCrossNamespaceNetworkPoliciesGetCreated()

			By("Patch Service")
			patch := client.MergeFrom(service.DeepCopy())
			service.Spec.Selector = nil
			Expect(testClient.Patch(ctx, service, patch)).To(Succeed())

			By("Wait until all policies are deleted")
			ensureNetworkPoliciesGetDeleted()
			ensureCrossNamespaceNetworkPoliciesGetDeleted()
		})

		It("should delete the policies when the service gets deleted", func() {
			By("Wait until all policies are created")
			ensureNetworkPoliciesGetCreated()
			ensureCrossNamespaceNetworkPoliciesGetCreated()

			By("Delete Service")
			Expect(testClient.Delete(ctx, service)).To(Succeed())

			By("Wait until all policies are deleted")
			ensureNetworkPoliciesGetDeleted()
			ensureCrossNamespaceNetworkPoliciesGetDeleted()
		})

		It("should delete the policies when the namespace is no longer handled", func() {
			By("Wait until all policies are created")
			ensureNetworkPoliciesGetCreated()
			ensureCrossNamespaceNetworkPoliciesGetCreated()

			By("Patch Namespace and remove label")
			patch := client.MergeFrom(otherNamespace.DeepCopy())
			otherNamespace.Labels["other"] = "namespace2"
			Expect(testClient.Patch(ctx, otherNamespace, patch)).To(Succeed())

			By("Wait until all policies are deleted")
			ensureNetworkPoliciesDoNotGetDeleted()
			ensureCrossNamespaceNetworkPoliciesGetDeleted()
		})

		It("should do nothing when the namespace is terminating", func() {
			By("Wait until all policies are created")
			ensureNetworkPoliciesGetCreated()

			DeferCleanup(func() {
				By("Remove finalizer from namespace to unblock deletion")
				patch := client.MergeFrom(namespace.DeepCopy())
				namespace.Finalizers = nil
				Expect(testClient.Patch(ctx, namespace, patch)).To(Succeed())
			})

			By("Add finalizer to namespace to block deletion")
			patch := client.MergeFrom(namespace.DeepCopy())
			namespace.Finalizers = append(namespace.Finalizers, finalizer)
			Expect(testClient.Patch(ctx, namespace, patch)).To(Succeed())

			By("Delete Namespace")
			Expect(testClient.Delete(ctx, namespace)).To(Succeed())

			By("Delete all NetworkPolicies")
			Expect(testClient.DeleteAllOf(ctx, &networkingv1.NetworkPolicy{}, client.InNamespace(namespace.Name))).To(Succeed())

			By("Reset log buffer")
			logBuffer.Reset()

			By("Add new port to Service (this should trigger the controller)")
			patch = client.MergeFrom(service.DeepCopy())
			service.Spec.Ports = append(service.Spec.Ports, corev1.ServicePort{Name: "newport", Port: 7636, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt32(6367)})
			Expect(testClient.Patch(ctx, service, patch)).To(Succeed())

			By("Ensure controller does not try to create new content in terminating namespace")
			Eventually(func() string { return logBuffer.String() }).ShouldNot(Or(
				ContainSubstring("unable to create new content in namespace"),
				ContainSubstring("because it is being terminated"),
			))
		})

		It("should create the expected cross-namespace policies as soon as a new namespace appears", func() {
			newNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
				GenerateName: "new-ns-",
				Labels:       map[string]string{"other": "namespace"},
			}}

			By("Create new Namespace")
			Expect(testClient.Create(ctx, newNamespace)).To(Succeed())
			log.Info("Created new Namespace", "namespace", client.ObjectKeyFromObject(newNamespace))

			DeferCleanup(func() {
				By("Delete new Namespace")
				Expect(testClient.Delete(ctx, newNamespace)).To(Or(Succeed(), BeNotFoundError()))
				log.Info("Deleted new Namespace", "namespace", client.ObjectKeyFromObject(newNamespace))

				By("Wait until manager has observed new Namespace deletion")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(newNamespace), newNamespace)
				}).Should(BeNotFoundError())
			})

			By("Wait until all ingress policies are created")
			Eventually(func(g Gomega) []string {
				networkPolicyList := &networkingv1.NetworkPolicyList{}
				g.Expect(testClient.List(ctx, networkPolicyList, client.InNamespace(service.Namespace))).To(Succeed())
				return test.ObjectNames(networkPolicyList)
			}).Should(ContainElements(
				"ingress-to-"+service.Name+port1Suffix+"-from-"+otherNamespace.Name,
				"ingress-to-"+service.Name+port2Suffix+"-from-"+otherNamespace.Name,
			))

			By("Wait until all egress policies are created")
			Eventually(func(g Gomega) []string {
				networkPolicyList := &networkingv1.NetworkPolicyList{}
				g.Expect(testClient.List(ctx, networkPolicyList, client.InNamespace(newNamespace.Name))).To(Succeed())
				return test.ObjectNames(networkPolicyList)
			}).Should(ContainElements(
				"egress-to-"+service.Namespace+"-"+service.Name+port1Suffix,
				"egress-to-"+service.Namespace+"-"+service.Name+port2Suffix,
			))
		})

		Context("with pod label selector namespace alias", func() {
			alias := "alias"

			BeforeEach(func() {
				metav1.SetMetaDataAnnotation(&service.ObjectMeta, "networking.resources.gardener.cloud/pod-label-selector-namespace-alias", alias)
			})

			It("should create the expected cross-namespace network policies", func() {
				ensureNetworkPoliciesGetCreated()

				By("Wait until ingress from other-namespace policy was created for first port")
				Eventually(func(g Gomega) *metav1.LabelSelector {
					networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "ingress-to-" + service.Name + port1Suffix + "-from-" + otherNamespace.Name, Namespace: service.Namespace}}
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
					return networkPolicy.Spec.Ingress[0].From[0].PodSelector
				}).Should(Equal(&metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + alias + "-" + service.Name + port1Suffix: "allowed"}}))

				By("Wait until egress from other-namespace policy was created for first port")
				Eventually(func(g Gomega) metav1.LabelSelector {
					networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "egress-to-" + service.Namespace + "-" + service.Name + port1Suffix, Namespace: otherNamespace.Name}}
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
					return networkPolicy.Spec.PodSelector
				}).Should(Equal(metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + alias + "-" + service.Name + port1Suffix: "allowed"}}))

				By("Wait until ingress from other-namespace policy was created for second port")
				Eventually(func(g Gomega) *metav1.LabelSelector {
					networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "ingress-to-" + service.Name + port2Suffix + "-from-" + otherNamespace.Name, Namespace: service.Namespace}}
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
					return networkPolicy.Spec.Ingress[0].From[0].PodSelector
				}).Should(Equal(&metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + alias + "-" + service.Name + port2Suffix: "allowed"}}))

				By("Wait until egress from other-namespace policy was created for second port")
				Eventually(func(g Gomega) metav1.LabelSelector {
					networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "egress-to-" + service.Namespace + "-" + service.Name + port2Suffix, Namespace: otherNamespace.Name}}
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
					return networkPolicy.Spec.PodSelector
				}).Should(Equal(metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + alias + "-" + service.Name + port2Suffix: "allowed"}}))
			})
		})
	})

	Context("service with custom pod label selectors", func() {
		BeforeEach(func() {
			metav1.SetMetaDataAnnotation(&service.ObjectMeta, "networking.resources.gardener.cloud/from-"+customPodLabelSelector1+"-allowed-ports", `[{"protocol":"`+string(port3Protocol)+`","port":"`+port3TargetPort.String()+`"},{"protocol":"`+string(port4Protocol)+`","port":`+port4TargetPort.String()+`}]`)
			metav1.SetMetaDataAnnotation(&service.ObjectMeta, "networking.resources.gardener.cloud/from-"+customPodLabelSelector2+"-allowed-ports", `[{"protocol":"`+string(port5Protocol)+`","port":`+port5TargetPort.String()+`},{"protocol":"`+string(port6Protocol)+`","port":`+port6TargetPort.String()+`}]`)
		})

		It("should create the expected network policies", func() {
			By("Wait until ingress policy was created for first port of first alias")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "ingress-to-" + service.Name + port3Suffix + "-via-" + customPodLabelSelector1, Namespace: service.Namespace}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
				PodSelector: metav1.LabelSelector{MatchLabels: serviceSelector},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{
					From:  []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + customPodLabelSelector1: "allowed"}}}},
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &port3Protocol, Port: &port3TargetPort}},
				}},
			}))

			By("Wait until egress policy was created for first port of first alias")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "egress-to-" + service.Name + port3Suffix + "-via-" + customPodLabelSelector1, Namespace: service.Namespace}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + customPodLabelSelector1: "allowed"}},
				Egress: []networkingv1.NetworkPolicyEgressRule{{
					To:    []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: serviceSelector}}},
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &port3Protocol, Port: &port3TargetPort}},
				}},
			}))

			By("Wait until ingress policy was created for second port of first alias")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "ingress-to-" + service.Name + port4Suffix + "-via-" + customPodLabelSelector1, Namespace: service.Namespace}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
				PodSelector: metav1.LabelSelector{MatchLabels: serviceSelector},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{
					From:  []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + customPodLabelSelector1: "allowed"}}}},
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &port4Protocol, Port: &port4TargetPort}},
				}},
			}))

			By("Wait until egress policy was created for second port of first alias")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "egress-to-" + service.Name + port4Suffix + "-via-" + customPodLabelSelector1, Namespace: service.Namespace}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + customPodLabelSelector1: "allowed"}},
				Egress: []networkingv1.NetworkPolicyEgressRule{{
					To:    []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: serviceSelector}}},
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &port4Protocol, Port: &port4TargetPort}},
				}},
			}))

			By("Wait until ingress policy was created for first port of second alias")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "ingress-to-" + service.Name + port5Suffix + "-via-" + customPodLabelSelector2, Namespace: service.Namespace}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
				PodSelector: metav1.LabelSelector{MatchLabels: serviceSelector},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{
					From:  []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + customPodLabelSelector2: "allowed"}}}},
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &port5Protocol, Port: &port5TargetPort}},
				}},
			}))

			By("Wait until egress policy was created for first port of second alias")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "egress-to-" + service.Name + port5Suffix + "-via-" + customPodLabelSelector2, Namespace: service.Namespace}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + customPodLabelSelector2: "allowed"}},
				Egress: []networkingv1.NetworkPolicyEgressRule{{
					To:    []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: serviceSelector}}},
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &port5Protocol, Port: &port5TargetPort}},
				}},
			}))

			By("Wait until ingress policy was created for second port of second alias")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "ingress-to-" + service.Name + port6Suffix + "-via-" + customPodLabelSelector2, Namespace: service.Namespace}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
				PodSelector: metav1.LabelSelector{MatchLabels: serviceSelector},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{
					From:  []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + customPodLabelSelector2: "allowed"}}}},
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &port6Protocol, Port: &port6TargetPort}},
				}},
			}))

			By("Wait until egress policy was created for second port of second alias")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "egress-to-" + service.Name + port6Suffix + "-via-" + customPodLabelSelector2, Namespace: service.Namespace}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"networking.resources.gardener.cloud/to-" + customPodLabelSelector2: "allowed"}},
				Egress: []networkingv1.NetworkPolicyEgressRule{{
					To:    []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: serviceSelector}}},
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &port6Protocol, Port: &port6TargetPort}},
				}},
			}))
		})

		It("should reconcile the policies when the allowed ports are changed", func() {
			By("Wait until all policies are created")
			ensureNetworkPoliciesWithCustomPodLabelSelectorsGetCreated()

			By("Patch Service")
			patch := client.MergeFrom(service.DeepCopy())
			metav1.SetMetaDataAnnotation(&service.ObjectMeta, "networking.resources.gardener.cloud/from-"+customPodLabelSelector1+"-allowed-ports", `[{"protocol":"`+string(port4Protocol)+`","port":`+port4TargetPort.String()+`},{"protocol":"`+string(corev1.ProtocolUDP)+`","port":2468}]`)
			Expect(testClient.Patch(ctx, service, patch)).To(Succeed())

			By("Wait until all policies were reconciled")
			Eventually(func(g Gomega) []string {
				networkPolicyList := &networkingv1.NetworkPolicyList{}
				g.Expect(testClient.List(ctx, networkPolicyList, client.InNamespace(service.Namespace))).To(Succeed())
				return test.ObjectNames(networkPolicyList)
			}).Should(And(
				Not(ContainElements(
					"ingress-to-"+service.Name+port3Suffix+"-via-"+customPodLabelSelector1,
					"egress-to-"+service.Name+port3Suffix+"-via-"+customPodLabelSelector1,
				)),
				ContainElements(
					"ingress-to-"+service.Name+port4Suffix+"-via-"+customPodLabelSelector1,
					"egress-to-"+service.Name+port4Suffix+"-via-"+customPodLabelSelector1,
					"ingress-to-"+service.Name+"-udp-2468-via-"+customPodLabelSelector1,
					"egress-to-"+service.Name+"-udp-2468-via-"+customPodLabelSelector1,
					"ingress-to-"+service.Name+port5Suffix+"-via-"+customPodLabelSelector2,
					"egress-to-"+service.Name+port5Suffix+"-via-"+customPodLabelSelector2,
					"ingress-to-"+service.Name+port6Suffix+"-via-"+customPodLabelSelector2,
					"egress-to-"+service.Name+port6Suffix+"-via-"+customPodLabelSelector2,
				),
			))
		})

		It("should not create any cross-namespace policies or ingress-from-world policy", func() {
			ensureCrossNamespaceNetworkPoliciesDoNotGetCreated()
			ensureIngressFromWorldNetworkPolicyDoesNotGetCreated()
		})

		It("should delete the policies when the custom pod label selectors in service annotations are removed", func() {
			By("Wait until all policies are created")
			ensureNetworkPoliciesWithCustomPodLabelSelectorsGetCreated()

			By("Patch Service")
			patch := client.MergeFrom(service.DeepCopy())
			delete(service.Annotations, "networking.resources.gardener.cloud/from-"+customPodLabelSelector1+"-allowed-ports")
			delete(service.Annotations, "networking.resources.gardener.cloud/from-"+customPodLabelSelector2+"-allowed-ports")
			Expect(testClient.Patch(ctx, service, patch)).To(Succeed())

			By("Wait until all policies are deleted")
			ensureNetworkPoliciesWithCustomPodLabelSelectorsGetDeleted()
		})

		It("should delete the policies when the service gets deleted", func() {
			By("Wait until all policies are created")
			ensureNetworkPoliciesWithCustomPodLabelSelectorsGetCreated()

			By("Delete Service")
			Expect(testClient.Delete(ctx, service)).To(Succeed())

			By("Wait until all policies are deleted")
			ensureNetworkPoliciesWithCustomPodLabelSelectorsGetDeleted()
		})

		It("should delete the policies when the namespace is no longer handled", func() {
			By("Wait until all policies are created")
			ensureNetworkPoliciesWithCustomPodLabelSelectorsGetCreated()

			By("Patch Namespace and remove label")
			patch := client.MergeFrom(namespace.DeepCopy())
			namespace.Labels[testID] = "foo"
			Expect(testClient.Patch(ctx, namespace, patch)).To(Succeed())

			By("Wait until all policies are deleted")
			ensureNetworkPoliciesWithCustomPodLabelSelectorsGetDeleted()
		})
	})

	Context("service with ingress from world", func() {
		BeforeEach(func() {
			metav1.SetMetaDataAnnotation(&service.ObjectMeta, "networking.resources.gardener.cloud/from-world-to-ports", `[{"port":`+port1TargetPort.String()+`,"protocol":"`+string(port1Protocol)+`"},{"port":"`+port2TargetPort.String()+`","protocol":"`+string(port2Protocol)+`"}]`)
		})

		It("should create the expected ingress-from-world network policy", func() {
			ensureNetworkPoliciesGetCreated()

			By("Wait until ingress from world policy was created")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "ingress-to-" + service.Name + "-from-world", Namespace: service.Namespace}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
				PodSelector: metav1.LabelSelector{MatchLabels: serviceSelector},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{
					Ports: []networkingv1.NetworkPolicyPort{
						{Protocol: &port1Protocol, Port: &port1TargetPort},
						{Protocol: &port2Protocol, Port: &port2TargetPort},
					},
				}},
			}))
		})

		It("should reconcile the policies when the ports in service are changed", func() {
			By("Wait until all policies are created")
			ensureIngressFromWorldNetworkPolicyGetsCreated()

			By("Patch Service")
			patch := client.MergeFrom(service.DeepCopy())
			metav1.SetMetaDataAnnotation(&service.ObjectMeta, "networking.resources.gardener.cloud/from-world-to-ports", "[]")
			Expect(testClient.Patch(ctx, service, patch)).To(Succeed())

			By("Wait until ingress from world policy was updated")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "ingress-to-" + service.Name + "-from-world", Namespace: service.Namespace}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
				PodSelector: metav1.LabelSelector{MatchLabels: serviceSelector},
				Ingress:     []networkingv1.NetworkPolicyIngressRule{{}},
			}))
		})

		It("should delete the policies when the pod selector in service is removed", func() {
			By("Wait until all policies are created")
			ensureNetworkPoliciesGetCreated()
			ensureIngressFromWorldNetworkPolicyGetsCreated()

			By("Patch Service")
			patch := client.MergeFrom(service.DeepCopy())
			service.Spec.Selector = nil
			Expect(testClient.Patch(ctx, service, patch)).To(Succeed())

			By("Wait until all policies are deleted")
			ensureNetworkPoliciesGetDeleted()
			ensureIngressFromWorldNetworkPolicyGetsDeleted()
		})

		It("should delete the policies when the service gets deleted", func() {
			By("Wait until all policies are created")
			ensureNetworkPoliciesGetCreated()
			ensureIngressFromWorldNetworkPolicyGetsCreated()

			By("Delete Service")
			Expect(testClient.Delete(ctx, service)).To(Succeed())

			By("Wait until all policies are deleted")
			ensureNetworkPoliciesGetDeleted()
			ensureIngressFromWorldNetworkPolicyGetsDeleted()
		})
	})

	Context("service exposed via ingress", func() {
		var (
			ensureExposedViaIngressNetworkPolicies = func(asyncAssertion func(int, any, ...any) AsyncAssertion, should bool) func() {
				return func() {
					assertedFunc := func(g Gomega) []networkingv1.NetworkPolicy {
						networkPolicyList := &networkingv1.NetworkPolicyList{}
						g.Expect(testClient.List(ctx, networkPolicyList)).To(Succeed())
						return networkPolicyList.Items
					}
					expectation := ContainElements(
						MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("ingress-to-" + service.Name + port1Suffix + "-from-ingress-controller"), "Namespace": Equal(service.Namespace)})}),
						MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("egress-to-" + service.Namespace + "-" + service.Name + port1Suffix + "-from-ingress-controller"), "Namespace": Equal(ingressControllerNamespace)})}),
					)

					if should {
						asyncAssertion(1, assertedFunc).Should(expectation)
					} else {
						asyncAssertion(1, assertedFunc).ShouldNot(expectation)
					}
				}
			}
			ensureExposedViaIngressNetworkPoliciesGetCreated = ensureExposedViaIngressNetworkPolicies(EventuallyWithOffset, true)
			ensureExposedViaIngressNetworkPoliciesGetDeleted = ensureExposedViaIngressNetworkPolicies(EventuallyWithOffset, false)

			controllerNamespace *corev1.Namespace
			ingress             *networkingv1.Ingress
		)

		JustBeforeEach(func() {
			controllerNamespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ingressControllerNamespace}}

			pathType := networkingv1.PathTypePrefix
			ingress = &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      service.Name,
					Namespace: service.Namespace,
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{
						Host: "foo.example.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path:     "/bar",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: service.Name,
											Port: networkingv1.ServiceBackendPort{
												Number: port1ServicePort,
											},
										},
									},
								}},
							},
						},
					}},
				},
			}

			By("Create ingress controller Namespace")
			Expect(testClient.Create(ctx, controllerNamespace)).To(Succeed())
			log.Info("Created ingress controller Namespace", "namespace", client.ObjectKeyFromObject(controllerNamespace))

			By("Create Ingress")
			Expect(testClient.Create(ctx, ingress)).To(Succeed())
			log.Info("Created Ingress", "ingress", client.ObjectKeyFromObject(ingress))

			DeferCleanup(func() {
				By("Delete Ingress")
				Expect(testClient.Delete(ctx, ingress)).To(Or(Succeed(), BeNotFoundError()))
				log.Info("Deleted Ingress", "ingress", client.ObjectKeyFromObject(ingress))

				By("Wait until manager has observed Ingress deletion")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(ingress), ingress)
				}).Should(BeNotFoundError())

				By("Delete ingress controller Namespace")
				Expect(testClient.Delete(ctx, controllerNamespace)).To(Or(Succeed(), BeNotFoundError()))
				log.Info("Deleted ingress controller Namespace", "namespace", client.ObjectKeyFromObject(controllerNamespace))

				By("Wait until manager has observed ingress controller Namespace deletion")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(controllerNamespace), controllerNamespace)
				}).Should(BeNotFoundError())
			})
		})

		It("should create the expected network policies", func() {
			By("Wait until ingress policy was created")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "ingress-to-" + service.Name + port1Suffix + "-from-ingress-controller", Namespace: service.Namespace}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
				PodSelector: metav1.LabelSelector{MatchLabels: serviceSelector},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{
					From: []networkingv1.NetworkPolicyPeer{{
						PodSelector:       &ingressControllerPodSelector,
						NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": controllerNamespace.Name}},
					}},
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &port1Protocol, Port: &port1TargetPort}},
				}},
			}))

			By("Wait until egress policy was created")
			Eventually(func(g Gomega) networkingv1.NetworkPolicySpec {
				networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "egress-to-" + service.Namespace + "-" + service.Name + port1Suffix + "-from-ingress-controller", Namespace: controllerNamespace.Name}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
				return networkPolicy.Spec
			}).Should(Equal(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				PodSelector: ingressControllerPodSelector,
				Egress: []networkingv1.NetworkPolicyEgressRule{{
					To: []networkingv1.NetworkPolicyPeer{{
						PodSelector:       &metav1.LabelSelector{MatchLabels: serviceSelector},
						NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": service.Namespace}},
					}},
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &port1Protocol, Port: &port1TargetPort}},
				}},
			}))
		})

		It("should reconcile the policies when the ports in service are changed", func() {
			By("Wait until all policies are created")
			ensureExposedViaIngressNetworkPoliciesGetCreated()

			By("Patch Service")
			newTargetPort := intstr.FromInt32(2468)
			patch := client.MergeFrom(service.DeepCopy())
			service.Spec.Ports[0].TargetPort = newTargetPort
			Expect(testClient.Patch(ctx, service, patch)).To(Succeed())

			By("Wait until all policies were reconciled")
			Eventually(func(g Gomega) []networkingv1.NetworkPolicy {
				networkPolicyList := &networkingv1.NetworkPolicyList{}
				g.Expect(testClient.List(ctx, networkPolicyList)).To(Succeed())
				return networkPolicyList.Items
			}).Should(And(
				Not(ContainElements(
					MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("ingress-to-" + service.Name + port1Suffix + "-from-ingress-controller"), "Namespace": Equal(service.Namespace)})}),
					MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("egress-to-" + service.Namespace + "-" + service.Name + port1Suffix + "-from-ingress-controller"), "Namespace": Equal(ingressControllerNamespace)})}),
				)),
				ContainElements(
					MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("ingress-to-" + service.Name + "-tcp-" + newTargetPort.String() + "-from-ingress-controller"), "Namespace": Equal(service.Namespace)})}),
					MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("egress-to-" + service.Namespace + "-" + service.Name + "-tcp-" + newTargetPort.String() + "-from-ingress-controller"), "Namespace": Equal(ingressControllerNamespace)})}),
				),
			))
		})

		It("should delete the policies when the pod selector in service is removed", func() {
			By("Wait until all policies are created")
			ensureExposedViaIngressNetworkPoliciesGetCreated()

			By("Patch Service")
			patch := client.MergeFrom(service.DeepCopy())
			service.Spec.Selector = nil
			Expect(testClient.Patch(ctx, service, patch)).To(Succeed())

			By("Wait until all policies are deleted")
			ensureExposedViaIngressNetworkPoliciesGetDeleted()
		})

		It("should delete the policies when the ingress gets deleted", func() {
			By("Wait until all policies are created")
			ensureExposedViaIngressNetworkPoliciesGetCreated()

			By("Delete Ingress")
			Expect(testClient.Delete(ctx, ingress)).To(Succeed())

			By("Wait until all policies are deleted")
			ensureExposedViaIngressNetworkPoliciesGetDeleted()
		})

		It("should delete the policies when the service gets deleted", func() {
			By("Wait until all policies are created")
			ensureExposedViaIngressNetworkPoliciesGetCreated()

			By("Delete Service")
			Expect(testClient.Delete(ctx, service)).To(Succeed())

			By("Wait until all policies are deleted")
			ensureExposedViaIngressNetworkPoliciesGetDeleted()
		})
	})
})
