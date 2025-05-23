// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/cache"
)

var _ = Describe("NetworkPolicy", func() {
	Describe("#NetworkPolicyToNodeExporter", func() {
		It("should return the expected network policy", func() {
			Expect(cache.NetworkPolicyToNodeExporter("foo", nil)).To(Equal(&networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "egress-from-cache-prometheus-to-kube-system-node-exporter-tcp-16909",
					Namespace: "foo",
				},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"prometheus": "cache"},
					},
					Egress: []networkingv1.NetworkPolicyEgressRule{{
						To:    []networkingv1.NetworkPolicyPeer{},
						Ports: []networkingv1.NetworkPolicyPort{{Port: ptr.To(intstr.FromInt32(16909)), Protocol: ptr.To(corev1.ProtocolTCP)}},
					}},
					Ingress:     []networkingv1.NetworkPolicyIngressRule{},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				},
			}))
		})

		It("should return the expected network policy with the node CIDR restriction", func() {
			var (
				nodeCIDR = "172.18.0.0/16"
			)
			Expect(cache.NetworkPolicyToNodeExporter("foo", &nodeCIDR)).To(Equal(&networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "egress-from-cache-prometheus-to-kube-system-node-exporter-tcp-16909",
					Namespace: "foo",
				},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"prometheus": "cache"},
					},
					Egress: []networkingv1.NetworkPolicyEgressRule{{
						To:    []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: nodeCIDR}}},
						Ports: []networkingv1.NetworkPolicyPort{{Port: ptr.To(intstr.FromInt32(16909)), Protocol: ptr.To(corev1.ProtocolTCP)}},
					}},
					Ingress:     []networkingv1.NetworkPolicyIngressRule{},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				},
			}))
		})
	})

	Describe("#NetworkPolicyToKubelet", func() {
		It("should return the expected network policy", func() {
			Expect(cache.NetworkPolicyToKubelet("foo", nil)).To(Equal(&networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "egress-from-cache-prometheus-to-kubelet-tcp-10250",
					Namespace: "foo",
				},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"prometheus": "cache"},
					},
					Egress: []networkingv1.NetworkPolicyEgressRule{{
						To:    []networkingv1.NetworkPolicyPeer{},
						Ports: []networkingv1.NetworkPolicyPort{{Port: ptr.To(intstr.FromInt32(10250)), Protocol: ptr.To(corev1.ProtocolTCP)}},
					}},
					Ingress:     []networkingv1.NetworkPolicyIngressRule{},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				},
			}))
		})

		It("should return the expected network policy with the node CIDR restriction", func() {
			var (
				nodeCIDR = "172.18.0.0/16"
			)
			Expect(cache.NetworkPolicyToKubelet("foo", &nodeCIDR)).To(Equal(&networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "egress-from-cache-prometheus-to-kubelet-tcp-10250",
					Namespace: "foo",
				},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"prometheus": "cache"},
					},
					Egress: []networkingv1.NetworkPolicyEgressRule{{
						To:    []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: nodeCIDR}}},
						Ports: []networkingv1.NetworkPolicyPort{{Port: ptr.To(intstr.FromInt32(10250)), Protocol: ptr.To(corev1.ProtocolTCP)}},
					}},
					Ingress:     []networkingv1.NetworkPolicyIngressRule{},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				},
			}))
		})
	})
})
