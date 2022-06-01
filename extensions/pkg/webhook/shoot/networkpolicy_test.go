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

package shoot_test

import (
	"context"

	. "github.com/gardener/gardener/extensions/pkg/webhook/shoot"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("NetworkPolicy", func() {
	var (
		ctx           = context.TODO()
		fakeClient    client.Client
		namespace     = "extension-foo-bar"
		extensionName = "provider-test"
		serverPort    = 1337
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	})

	Describe("#EnsureNetworkPolicy", func() {
		It("should reconcile the correct network policy", func() {
			Expect(EnsureNetworkPolicy(ctx, fakeClient, namespace, extensionName, serverPort)).To(Succeed())

			networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "gardener-extension-" + extensionName}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())

			port := intstr.FromInt(serverPort)
			protocol := corev1.ProtocolTCP

			Expect(networkPolicy.Spec).To(DeepEqual(networkingv1.NetworkPolicySpec{
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					{
						Ports: []networkingv1.NetworkPolicyPort{
							{
								Port:     &port,
								Protocol: &protocol,
							},
						},
						To: []networkingv1.NetworkPolicyPeer{
							{
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.GardenRole: v1beta1constants.GardenRoleExtension,
									},
								},
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"app.kubernetes.io/name": "gardener-extension-" + extensionName,
									},
								},
							},
						},
					},
				},
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
						v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer,
					},
				},
			}))
		})
	})
})
