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

package helper_test

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy/helper"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("helper", func() {
	Describe("#GetEgressRules", func() {
		It("should return and empty EgressRule", func() {
			Expect(GetEgressRules()).To(BeEmpty())
		})

		It("should return the Egress rule with correct IP Blocks", func() {
			var (
				ip1     = "10.250.119.142"
				ip2     = "10.250.119.143"
				ip3     = "10.250.119.144"
				ip4     = "10.250.119.145"
				subsets = []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: ip1,
							},
							{
								IP: ip2,
							},
							{
								IP: ip2, // duplicate address should be removed
							},
						},
					},
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: ip3,
							},
							{
								IP: ip4,
							},
							{
								IP: ip2, // duplicate address should be removed
							},
							{
								IP: ip4, // duplicate address should be removed
							},
						},
						NotReadyAddresses: []corev1.EndpointAddress{
							{
								IP: "10.250.119.146",
							},
						},
					},
				}
			)

			egressRules := GetEgressRules(subsets...)
			expectedRules := []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: fmt.Sprintf("%s/32", ip1),
							},
						},
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: fmt.Sprintf("%s/32", ip2),
							},
						},
					},
				},
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: fmt.Sprintf("%s/32", ip3),
							},
						},
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: fmt.Sprintf("%s/32", ip4),
							},
						},
					},
				},
			}
			Expect(egressRules).To(Equal(expectedRules))
		})

		It("should return the Egress rule with correct Ports", func() {
			var (
				tcp     = corev1.ProtocolTCP
				udp     = corev1.ProtocolUDP
				subsets = []corev1.EndpointSubset{
					{
						Ports: []corev1.EndpointPort{
							{
								Protocol: tcp,
								Port:     443,
							},
						},
					},
					{
						Ports: []corev1.EndpointPort{
							{
								Protocol: corev1.ProtocolUDP,
								Port:     161,
							},
						},
					},
				}
			)

			egressRules := GetEgressRules(subsets...)
			port443 := intstr.FromInt(443)
			port161 := intstr.FromInt(161)
			expectedRules := []networkingv1.NetworkPolicyEgressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &tcp,
							Port:     &port443,
						},
					},
				},
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &udp,
							Port:     &port161,
						},
					},
				},
			}
			Expect(egressRules).To(Equal(expectedRules))
		})
	})

	Describe("#EnsureNetworkPolicy", func() {
		var (
			ctrl              *gomock.Controller
			mockRuntimeClient = mockclient.NewMockClient(ctrl)
			ctx               = context.TODO()
			tcp               = corev1.ProtocolTCP
			port443           = intstr.FromInt(443)
			namespace         = "shoot-ns"

			expectedRules = []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: fmt.Sprintf("%s/32", "10.250.119.142"),
							},
						},
					},
				},
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &tcp,
							Port:     &port443,
						},
					},
				},
			}
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockRuntimeClient = mockclient.NewMockClient(ctrl)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should create the allow-to-seed-apiserver Network Policy", func() {
			mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(namespace, AllowToSeedAPIServer), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})).Return(errors.NewNotFound(core.Resource("networkpolicy"), ""))
			mockRuntimeClient.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})).DoAndReturn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
				policy, ok := obj.(*networkingv1.NetworkPolicy)
				Expect(ok).To(BeTrue())
				Expect(policy.Annotations).To(HaveKeyWithValue("gardener.cloud/description", "Allows Egress from pods labeled with 'networking.gardener.cloud/to-seed-apiserver=allowed' to Seed's Kubernetes API Server endpoints in the default namespace."))
				Expect(policy.Spec).To(Equal(networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyToSeedAPIServer: v1beta1constants.LabelNetworkPolicyAllowed},
					},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
					Egress:      expectedRules,
					Ingress:     []networkingv1.NetworkPolicyIngressRule{},
				}))
				return nil
			})

			err := EnsureNetworkPolicy(ctx, mockRuntimeClient, namespace, expectedRules)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should update the allow-to-seed-apiserver Network Policy", func() {
			mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(namespace, AllowToSeedAPIServer), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})).Return(nil)
			mockRuntimeClient.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()).
				DoAndReturn(func(_ context.Context, policy *networkingv1.NetworkPolicy, _ client.Patch, _ ...client.PatchOption) error {
					Expect(policy.Annotations).To(HaveKeyWithValue("gardener.cloud/description", "Allows Egress from pods labeled with 'networking.gardener.cloud/to-seed-apiserver=allowed' to Seed's Kubernetes API Server endpoints in the default namespace."))
					Expect(policy.Spec).To(Equal(networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyToSeedAPIServer: v1beta1constants.LabelNetworkPolicyAllowed},
						},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
						Egress:      expectedRules,
						Ingress:     []networkingv1.NetworkPolicyIngressRule{},
					}))
					return nil
				})

			err := EnsureNetworkPolicy(ctx, mockRuntimeClient, namespace, expectedRules)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
