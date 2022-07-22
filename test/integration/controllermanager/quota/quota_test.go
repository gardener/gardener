// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Quota controller tests", func() {
	var (
		providerType string
		resourceName string
		objectKey    client.ObjectKey

		secret        *corev1.Secret
		quota         *gardencorev1beta1.Quota
		secretbinding *gardencorev1beta1.SecretBinding
	)

	BeforeEach(func() {
		providerType = "provider-type"
		resourceName = "test-" + utils.ComputeSHA256Hex([]byte(CurrentSpecReport().LeafNodeLocation.String()))[:8]
		objectKey = client.ObjectKey{Namespace: testNamespace.Name, Name: resourceName}

		secret = &corev1.Secret{
			ObjectMeta: kutil.ObjectMetaFromKey(objectKey),
		}

		quota = &gardencorev1beta1.Quota{
			ObjectMeta: kutil.ObjectMetaFromKey(objectKey),
			Spec: gardencorev1beta1.QuotaSpec{
				Scope: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
				},
			},
		}

		secretbinding = &gardencorev1beta1.SecretBinding{
			ObjectMeta: kutil.ObjectMetaFromKey(objectKey),
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
		log.Info("Created secret for test", "secret", client.ObjectKeyFromObject(secret))

		By("Create Quota")
		Expect(testClient.Create(ctx, quota)).To(Succeed())
		log.Info("Created quota for test", "quota", client.ObjectKeyFromObject(quota))

		DeferCleanup(func() {
			By("Delete Quota")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, quota))).To(Succeed())
		})

		if secretbinding != nil {
			By("Create SecretBinding")
			Expect(testClient.Create(ctx, secretbinding)).To(Succeed())
			log.Info("Created secretbinding for test", "secretbinding", client.ObjectKeyFromObject(secretbinding))

			DeferCleanup(func() {
				By("Delete SecretBinding")
				Expect(client.IgnoreNotFound(gardener.ConfirmDeletion(ctx, testClient, secretbinding))).To(Succeed())
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, secretbinding))).To(Succeed())
			})
		}
	})

	Context("no secretbinding referencing Quota", func() {
		BeforeEach(func() {
			secretbinding = nil
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

	Context("secretbinding referencing Quota", func() {
		JustBeforeEach(func() {
			By("Ensure finalizer got added")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, objectKey, quota)).To(Succeed())
				g.Expect(quota.Finalizers).To(ConsistOf("gardener"))
			}).Should(Succeed())

			By("Delete Quota")
			Expect(testClient.Delete(ctx, quota)).To(Succeed())
		})

		It("should add finalizer and not release it on deletion since there is still referencing secretbinding", func() {
			By("Ensure Quota is not released")
			Consistently(func() error {
				return testClient.Get(ctx, objectKey, quota)
			}).Should(Succeed())
		})

		It("should add the finalizer and release it on deletion after secretbinding got deleted", func() {
			By("Delete Secretbinding")
			Expect(gardener.ConfirmDeletion(ctx, testClient, secretbinding)).To(Succeed())
			Expect(testClient.Delete(ctx, secretbinding)).To(Succeed())

			By("Ensure Quota is released")
			Eventually(func() error {
				return testClient.Get(ctx, objectKey, quota)
			}).Should(BeNotFoundError())
		})
	})
})
