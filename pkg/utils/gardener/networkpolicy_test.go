// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("NetworkPolicy", func() {
	Describe("#InjectNetworkPolicyAnnotationsForScrapeTargets", func() {
		It("should inject the annotations", func() {
			obj := &corev1.Service{}

			Expect(InjectNetworkPolicyAnnotationsForScrapeTargets(
				obj,
				networkingv1.NetworkPolicyPort{Port: ptr.To(intstr.FromInt32(1234)), Protocol: ptr.To(corev1.ProtocolTCP)},
				networkingv1.NetworkPolicyPort{Port: ptr.To(intstr.FromString("foo")), Protocol: ptr.To(corev1.ProtocolUDP)},
			)).Should(Succeed())

			Expect(obj.Annotations).To(And(
				HaveKeyWithValue("networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports", `[{"protocol":"TCP","port":1234},{"protocol":"UDP","port":"foo"}]`),
			))
		})
	})

	Describe("#InjectNetworkPolicyAnnotationsForGardenScrapeTargets", func() {
		It("should inject the annotations", func() {
			obj := &corev1.Service{}

			Expect(InjectNetworkPolicyAnnotationsForGardenScrapeTargets(
				obj,
				networkingv1.NetworkPolicyPort{Port: ptr.To(intstr.FromInt32(1234)), Protocol: ptr.To(corev1.ProtocolTCP)},
				networkingv1.NetworkPolicyPort{Port: ptr.To(intstr.FromString("foo")), Protocol: ptr.To(corev1.ProtocolUDP)},
			)).Should(Succeed())

			Expect(obj.Annotations).To(And(
				HaveKeyWithValue("networking.resources.gardener.cloud/from-all-garden-scrape-targets-allowed-ports", `[{"protocol":"TCP","port":1234},{"protocol":"UDP","port":"foo"}]`),
			))
		})
	})

	Describe("#InjectNetworkPolicyAnnotationsForSeedScrapeTargets", func() {
		It("should inject the annotations", func() {
			obj := &corev1.Service{}

			Expect(InjectNetworkPolicyAnnotationsForSeedScrapeTargets(
				obj,
				networkingv1.NetworkPolicyPort{Port: ptr.To(intstr.FromInt32(1234)), Protocol: ptr.To(corev1.ProtocolTCP)},
				networkingv1.NetworkPolicyPort{Port: ptr.To(intstr.FromString("foo")), Protocol: ptr.To(corev1.ProtocolUDP)},
			)).Should(Succeed())

			Expect(obj.Annotations).To(And(
				HaveKeyWithValue("networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports", `[{"protocol":"TCP","port":1234},{"protocol":"UDP","port":"foo"}]`),
			))
		})
	})

	Describe("#InjectNetworkPolicyAnnotationsForWebhookTargets", func() {
		It("should inject the annotations", func() {
			obj := &corev1.Service{}

			Expect(InjectNetworkPolicyAnnotationsForWebhookTargets(
				obj,
				networkingv1.NetworkPolicyPort{Port: ptr.To(intstr.FromInt32(1234)), Protocol: ptr.To(corev1.ProtocolTCP)},
				networkingv1.NetworkPolicyPort{Port: ptr.To(intstr.FromString("foo")), Protocol: ptr.To(corev1.ProtocolUDP)},
			)).Should(Succeed())

			Expect(obj.Annotations).To(And(
				HaveKeyWithValue("networking.resources.gardener.cloud/from-all-webhook-targets-allowed-ports", `[{"protocol":"TCP","port":1234},{"protocol":"UDP","port":"foo"}]`),
			))
		})
	})

	Describe("#InjectNetworkPolicyNamespaceSelectors", func() {
		It("should inject the annotation", func() {
			obj := &corev1.Service{}

			Expect(InjectNetworkPolicyNamespaceSelectors(
				obj,
				metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "foo", Operator: metav1.LabelSelectorOpIn, Values: []string{"bar"}}}},
			)).Should(Succeed())

			Expect(obj.Annotations).To(HaveKeyWithValue("networking.resources.gardener.cloud/namespace-selectors", `[{"matchLabels":{"foo":"bar"}},{"matchExpressions":[{"key":"foo","operator":"In","values":["bar"]}]}]`))
		})
	})

	Describe("#NetworkPolicyLabel", func() {
		It("should return the expected value", func() {
			Expect(NetworkPolicyLabel("foo", 1234)).To(Equal("networking.resources.gardener.cloud/to-foo-tcp-1234"))
		})
	})
})
