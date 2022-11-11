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
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	networkpolicyhelper "github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy/helper"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Network policy controller tests", func() {
	const allowToSeedAPIServer = "allow-to-seed-apiserver"

	var (
		expectedNetworkPolicySpec networkingv1.NetworkPolicySpec
		shootNamespace            *corev1.Namespace
		fooNamespace              *corev1.Namespace
		kubernetesEndpoint        *corev1.Endpoints
	)

	BeforeEach(func() {
		kubernetesEndpoint = &corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "kubernetes",
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
		By("Construct expected network policy")
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(kubernetesEndpoint), kubernetesEndpoint)).To(Succeed())

		expectedNetworkPolicySpec = networkingv1.NetworkPolicySpec{
			Egress: networkpolicyhelper.GetEgressRules(kubernetesEndpoint.Subsets...),
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"networking.gardener.cloud/to-seed-apiserver": "allowed"},
			},
			PolicyTypes: []networkingv1.PolicyType{"Egress"},
		}

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

	Context("reconciliation", func() {
		It("should create the network policy in the shoot namespace", func() {
			By("Wait for controller to reconcile the network policy")
			Eventually(func(g Gomega) {
				shootNetworkPolicy := &networkingv1.NetworkPolicy{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: shootNamespace.Name, Name: allowToSeedAPIServer}, shootNetworkPolicy)).To(Succeed())
				g.Expect(shootNetworkPolicy.Spec).To(Equal(expectedNetworkPolicySpec))
			}).Should(Succeed())
		})

		It("should create the network policy in the garden namespace", func() {
			By("Wait for controller to reconcile the network policy")
			Eventually(func(g Gomega) {
				gardenNetworkPolicy := &networkingv1.NetworkPolicy{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace.Name, Name: allowToSeedAPIServer}, gardenNetworkPolicy)).To(Succeed())
				g.Expect(gardenNetworkPolicy.Spec).To(Equal(expectedNetworkPolicySpec))
			}).Should(Succeed())
		})

		It("should create the network policy in the istio-system namespace", func() {
			By("Wait for controller to reconcile the network policy")
			Eventually(func(g Gomega) {
				istioSystemNetworkPolicy := &networkingv1.NetworkPolicy{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: istioSystemNamespace.Name, Name: allowToSeedAPIServer}, istioSystemNetworkPolicy)).To(Succeed())
				g.Expect(istioSystemNetworkPolicy.Spec).To(Equal(expectedNetworkPolicySpec))
			}).Should(Succeed())
		})

		It("should not create the network policy in the foo namespace", func() {
			By("Wait for controller to reconcile the network policy")
			Consistently(func(g Gomega) {
				fooNetworkPolicy := &networkingv1.NetworkPolicy{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: fooNamespace.Name, Name: allowToSeedAPIServer}, fooNetworkPolicy)).Should(BeNotFoundError())
			}).Should(Succeed())
		})

		It("should reconcile the network policy in the shoot namespace when it is changed by a third party", func() {
			By("Modify network policy in shoot namespace")
			modifiedShootNetworkPolicy := &networkingv1.NetworkPolicy{}
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKey{Namespace: shootNamespace.Name, Name: allowToSeedAPIServer}, modifiedShootNetworkPolicy)
			}).Should(Succeed())
			modifiedShootNetworkPolicy.Spec.PodSelector.MatchLabels["foo"] = "bar"
			Expect(testClient.Update(ctx, modifiedShootNetworkPolicy)).To(Succeed())

			By("Wait for controller to reconcile the network policy")
			Eventually(func(g Gomega) {
				shootNetworkPolicy := &networkingv1.NetworkPolicy{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: shootNamespace.Name, Name: allowToSeedAPIServer}, shootNetworkPolicy)).To(Succeed())
				g.Expect(shootNetworkPolicy.Spec).To(Equal(expectedNetworkPolicySpec))
			}).Should(Succeed())
		})

		It("should not update the network policy if nothing changed", func() {
			By("Modify shoot namespace to trigger reconciliation")
			beforeShootNetworkPolicy := &networkingv1.NetworkPolicy{}
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKey{Namespace: shootNamespace.Name, Name: allowToSeedAPIServer}, beforeShootNetworkPolicy)
			}).Should(Succeed())
			shootNamespace.Labels["foo"] = "bar"
			Expect(testClient.Update(ctx, shootNamespace)).To(Succeed())

			By("Wait for controller to reconcile the network policy")
			Consistently(func(g Gomega) {
				shootNetworkPolicy := &networkingv1.NetworkPolicy{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: shootNamespace.Name, Name: allowToSeedAPIServer}, shootNetworkPolicy)).To(Succeed())
				g.Expect(shootNetworkPolicy.Generation).To(Equal(beforeShootNetworkPolicy.Generation))
			}).Should(Succeed())
		})
	})

	Context("deletion", func() {
		It("should delete the network policy in foo namespace", func() {
			fooNetworkPolicy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: fooNamespace.Name,
					Name:      allowToSeedAPIServer,
				},
				Spec: expectedNetworkPolicySpec,
			}
			By("Create foo network policy")
			Expect(testClient.Create(ctx, fooNetworkPolicy)).To(Succeed())

			DeferCleanup(func() {
				By("Delete foo network policy")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, fooNetworkPolicy))).To(Succeed())
			})

			By("Wait for controller to delete the network policy")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(fooNetworkPolicy), fooNetworkPolicy)).Should(BeNotFoundError())
			}).Should(Succeed())
		})
	})
})
