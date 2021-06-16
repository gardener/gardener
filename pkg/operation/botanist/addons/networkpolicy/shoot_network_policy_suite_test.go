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

package networkpolicy_test

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	cr "github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/operation/botanist/addons/networkpolicy"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNetworkPolicy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shoot NetworkPolicy Suite")
}

const (
	chartsRootPath = "../../../../../charts"
)

var _ = Describe("Shoot NetworkPolicy Chart", func() {
	var (
		c            client.Client
		ctx          context.Context
		np, expected *networkingv1.NetworkPolicy
		val          ShootNetworkPolicyValues
	)

	BeforeEach(func() {
		var (
			tcp      = corev1.ProtocolTCP
			udp      = corev1.ProtocolUDP
			port53   = intstr.FromInt(53)
			port8053 = intstr.FromInt(8053)
			s        = runtime.NewScheme()
		)

		ctx = context.TODO()
		np = &networkingv1.NetworkPolicy{}
		val = ShootNetworkPolicyValues{Enabled: true}
		expected = &networkingv1.NetworkPolicy{
			TypeMeta: metav1.TypeMeta{
				Kind:       "NetworkPolicy",
				APIVersion: "networking.k8s.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud--allow-to-dns",
				Namespace: "kube-system",
				Labels:    map[string]string{"origin": "gardener"},
				Annotations: map[string]string{
					"gardener.cloud/description": "Allows Egress from pods labeled with 'networking.gardener.cloud/to-dns=allowed'\nto DNS running in the 'kube-system' namespace.\n",
				},
				ResourceVersion: "1",
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"networking.gardener.cloud/to-dns": "allowed",
					},
				},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					{
						Ports: []networkingv1.NetworkPolicyPort{
							{Protocol: &udp, Port: &port8053},
							{Protocol: &tcp, Port: &port8053},
						},
						To: []networkingv1.NetworkPolicyPeer{
							{
								PodSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      "k8s-app",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"kube-dns"},
									}},
								},
							},
						},
					},
					{
						Ports: []networkingv1.NetworkPolicyPort{
							{Protocol: &udp, Port: &port53},
							{Protocol: &tcp, Port: &port53},
						},
						To: []networkingv1.NetworkPolicyPeer{{
							IPBlock: &networkingv1.IPBlock{
								CIDR: "0.0.0.0/0",
							},
						}},
					},
				},
			},
		}

		Expect(networkingv1.AddToScheme(s)).NotTo(HaveOccurred(), "adding to schema succeeds")
		c = fakeclient.NewClientBuilder().WithScheme(s).Build()
	})

	JustBeforeEach(func() {
		ca := kubernetes.NewChartApplier(
			cr.NewWithServerVersion(&version.Info{}),
			kubernetes.NewApplier(c, meta.NewDefaultRESTMapper([]schema.GroupVersion{})),
		)

		Expect(ca.Apply(
			ctx,
			filepath.Join(chartsRootPath, "shoot-core", "components", "charts", "network-policies"),
			"kube-system",
			"bar",
			kubernetes.Values(val),
		)).NotTo(HaveOccurred(), "can apply chart")

		err := c.Get(ctx, client.ObjectKey{Name: "gardener.cloud--allow-to-dns", Namespace: "kube-system"}, np)
		Expect(err).ToNot(HaveOccurred(), "get succeeds")
	})

	Context("nodelocaldns is disabled", func() {
		BeforeEach(func() {
			val.NodeLocalDNS.Enabled = false
		})

		It("allows traffic only to coredns", func() {
			Expect(np).To(Equal(expected))
		})
	})

	Context("nodelocaldns is enabled", func() {
		BeforeEach(func() {
			val.NodeLocalDNS.Enabled = true
			val.NodeLocalDNS.KubeDNSClusterIP = "1.2.3.4"
		})

		It("allows traffic only to coredns", func() {
			expected.Spec.Egress[0].To = append(expected.Spec.Egress[0].To, networkingv1.NetworkPolicyPeer{
				IPBlock: &networkingv1.IPBlock{
					CIDR: "1.2.3.4/32",
				},
			})
			Expect(np).To(Equal(expected))
		})
	})
})
