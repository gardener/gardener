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

package quota_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Quota controller tests", func() {
	var (
		providerType string
		resourceName string
		objectKey    client.ObjectKey

		secret        *corev1.Secret
		quota         *gardencorev1beta1.Quota
		secretBinding *gardencorev1beta1.SecretBinding
	)

	BeforeEach(func() {
		providerType = "provider-type"
		resourceName = "test-" + gardenerutils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]
		objectKey = client.ObjectKey{Namespace: testNamespace.Name, Name: resourceName}

		secret = &corev1.Secret{
			ObjectMeta: kubernetesutils.ObjectMetaFromKey(objectKey),
		}

		quota = &gardencorev1beta1.Quota{
			ObjectMeta: kubernetesutils.ObjectMetaFromKey(objectKey),
			Spec: gardencorev1beta1.QuotaSpec{
				Scope: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
				},
			},
		}

		secretBinding = &gardencorev1beta1.SecretBinding{
			ObjectMeta: kubernetesutils.ObjectMetaFromKey(objectKey),
			Provider: &gardencorev1beta1.SecretBindingProvider{
				Type: providerType,
			},
			SecretRef: corev1.SecretReference{
				Name:      resourceName,
				Namespace: testNamespace.Name,
			},
			Quotas: []corev1.ObjectReference{
				{
					Name:      resourceName,
					Namespace: testNamespace.Name,
				},
			},
		}
	})

	JustBeforeEach(func() {
		By("Create Secret")
		Expect(testClient.Create(ctx, secret)).To(Succeed())
		log.Info("Created Secret for test", "secret", client.ObjectKeyFromObject(secret))

		DeferCleanup(func() {
			By("Delete Secret")
			Expect(testClient.Delete(ctx, secret)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Create Quota")
		Expect(testClient.Create(ctx, quota)).To(Succeed())
		log.Info("Created Quota for test", "quota", client.ObjectKeyFromObject(quota))

		DeferCleanup(func() {
			By("Delete Quota")
			Expect(testClient.Delete(ctx, quota)).To(Or(Succeed(), BeNotFoundError()))
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(quota), quota)
			}).Should(BeNotFoundError())
		})

		if secretBinding != nil {
			By("Create SecretBinding")
			Expect(testClient.Create(ctx, secretBinding)).To(Succeed())
			log.Info("Created SecretBinding for test", "secretBinding", client.ObjectKeyFromObject(secretBinding))

			By("Wait until manager has observed SecretBinding")
			// Use the manager's cache to ensure it has observed the SecretBinding.
			// Otherwise, the controller might clean up the Quota too early because it thinks all referencing
			// SecretBindings are gone. Similar to https://github.com/gardener/gardener/issues/6486 and
			// https://github.com/gardener/gardener/issues/6607.
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(secretBinding), &gardencorev1beta1.SecretBinding{})
			}).Should(Succeed())

			DeferCleanup(func() {
				By("Delete SecretBinding")
				Expect(testClient.Delete(ctx, secretBinding)).To(Or(Succeed(), BeNotFoundError()))
			})
		}
	})

	Context("no SecretBinding referencing Quota", func() {
		BeforeEach(func() {
			secretBinding = nil
		})

		It("should add the finalizer and release it on deletion", func() {
			By("Ensure finalizer got added")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, objectKey, quota)).To(Succeed())
				g.Expect(quota.Finalizers).To(ConsistOf("gardener"))
			}).Should(Succeed())

			By("Delete Quota")
			Expect(testClient.Delete(ctx, quota)).To(Succeed())

			By("Ensure Quota is released")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(quota), quota)
			}).Should(BeNotFoundError())
		})
	})

	Context("SecretBinding referencing Quota", func() {
		JustBeforeEach(func() {
			By("Ensure finalizer got added")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, objectKey, quota)).To(Succeed())
				g.Expect(quota.Finalizers).To(ConsistOf("gardener"))
			}).Should(Succeed())

			By("Delete Quota")
			Expect(testClient.Delete(ctx, quota)).To(Succeed())
		})

		It("should add finalizer and not release it on deletion since there is still referencing SecretBinding", func() {
			By("Ensure Quota is not released")
			Consistently(func() error {
				return testClient.Get(ctx, objectKey, quota)
			}).Should(Succeed())
		})

		It("should add the finalizer and release it on deletion after SecretBinding got deleted", func() {
			By("Delete SecretBinding")
			Expect(testClient.Delete(ctx, secretBinding)).To(Succeed())

			By("Ensure Quota is released")
			Eventually(func() error {
				return testClient.Get(ctx, objectKey, quota)
			}).Should(BeNotFoundError())
		})
	})
})
