// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package secrets_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Seed Secrets controller tests", func() {
	var (
		seed    *gardencorev1beta1.Seed
		secret1 *corev1.Secret
		secret2 *corev1.Secret
		secret3 *corev1.Secret
		secret4 *corev1.Secret
	)

	BeforeEach(func() {
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.SeedSpec{
				Provider: gardencorev1beta1.SeedProvider{
					Region: "region",
					Type:   "providerType",
				},
				Ingress: &gardencorev1beta1.Ingress{
					Domain: "someingress.example.com",
					Controller: gardencorev1beta1.IngressController{
						Kind: "nginx",
					},
				},
				DNS: gardencorev1beta1.SeedDNS{
					Provider: &gardencorev1beta1.SeedDNSProvider{
						Type: "providerType",
						SecretRef: corev1.SecretReference{
							Name:      "some-secret",
							Namespace: "some-namespace",
						},
					},
				},
				Networks: gardencorev1beta1.SeedNetworks{
					Pods:     "10.0.0.0/16",
					Services: "10.1.0.0/16",
					Nodes:    pointer.String("10.2.0.0/16"),
					ShootDefaults: &gardencorev1beta1.ShootNetworks{
						Pods:     pointer.String("100.128.0.0/11"),
						Services: pointer.String("100.72.0.0/13"),
					},
				},
			},
		}

		secret1 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "secret-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{"gardener.cloud/role": "foo"},
			},
		}
		secret2 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "secret-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{"gardener.cloud/role": "foo"},
			},
		}
		secret3 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "secret-",
				Namespace:    testNamespace.Name,
			},
		}
		secret4 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "secret-",
				Namespace:    metav1.NamespaceDefault,
			},
		}

		Expect(testClient.Create(ctx, secret1)).To(Succeed())
		Expect(testClient.Create(ctx, secret2)).To(Succeed())
		Expect(testClient.Create(ctx, secret3)).To(Succeed())
		Expect(testClient.Create(ctx, secret4)).To(Succeed())

		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, secret1)).To(Or(Succeed(), BeNotFoundError()))
			Expect(testClient.Delete(ctx, secret2)).To(Or(Succeed(), BeNotFoundError()))
			Expect(testClient.Delete(ctx, secret3)).To(Or(Succeed(), BeNotFoundError()))
			Expect(testClient.Delete(ctx, secret4)).To(Or(Succeed(), BeNotFoundError()))
		})
	})

	It("should create a seed namespace and sync the secrets properly", func() {
		By("Create Seed")
		Expect(testClient.Create(ctx, seed)).To(Succeed())

		By("Expect seed namespace to be created")
		Eventually(func(g Gomega) {
			namespace := &corev1.Namespace{}
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "seed-" + seed.Name}, namespace)).To(Succeed())
			g.Expect(namespace.OwnerReferences).To(HaveLen(1))
			g.Expect(namespace.OwnerReferences[0].Kind).To(Equal("Seed"))
			g.Expect(namespace.OwnerReferences[0].Name).To(Equal(seed.Name))
		}).Should(Succeed())

		By("Expect relevant garden secrets to be synced to seed namespace")
		Eventually(func(g Gomega) {
			secretList := &corev1.SecretList{}
			g.Expect(testClient.List(ctx, secretList, client.InNamespace("seed-"+seed.Name))).To(Succeed())
			g.Expect(secretList.Items).To(ConsistOf(
				MatchFields(IgnoreExtras, Fields{
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{
						"Name": Equal(secret1.Name),
					}),
				}),
				MatchFields(IgnoreExtras, Fields{
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{
						"Name": Equal(secret2.Name),
					}),
				}),
			))
		}).Should(Succeed())

		By("Delete relevant garden secret")
		Expect(testClient.Delete(ctx, secret1)).To(Succeed())

		By("Expect deleted secret to also be deleted from seed namespace")
		Eventually(func(g Gomega) {
			secretList := &corev1.SecretList{}
			g.Expect(testClient.List(ctx, secretList, client.InNamespace("seed-"+seed.Name))).To(Succeed())
			g.Expect(secretList.Items).To(ConsistOf(
				MatchFields(IgnoreExtras, Fields{
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{
						"Name": Equal(secret2.Name),
					}),
				}),
			))
		}).Should(Succeed())

		By("Change relevant garden secret")
		patch := client.MergeFrom(secret2.DeepCopy())
		metav1.SetMetaDataLabel(&secret2.ObjectMeta, "foo", "bar")
		Expect(testClient.Patch(ctx, secret2, patch)).To(Succeed())

		By("Expect changes to reflect in secret in seed namespace")
		Eventually(func(g Gomega) {
			secretList := &corev1.SecretList{}
			g.Expect(testClient.List(ctx, secretList, client.InNamespace("seed-"+seed.Name))).To(Succeed())
			g.Expect(secretList.Items).To(ConsistOf(
				MatchFields(IgnoreExtras, Fields{
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{
						"Name":   Equal(secret2.Name),
						"Labels": HaveKeyWithValue("foo", "bar"),
					}),
				}),
			))
		}).Should(Succeed())
	})
})
