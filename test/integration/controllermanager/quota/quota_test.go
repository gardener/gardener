// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package quota_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Quota controller tests", func() {
	var (
		providerType string
		resourceName string
		objectKey    client.ObjectKey

		secret             *corev1.Secret
		quota              *gardencorev1beta1.Quota
		secretBinding      *gardencorev1beta1.SecretBinding
		credentialsBinding *securityv1alpha1.CredentialsBinding
	)

	BeforeEach(func() {
		providerType = "provider-type"
		resourceName = "test-" + gardenerutils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]
		objectKey = client.ObjectKey{Namespace: testNamespace.Name, Name: resourceName}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: objectKey.Namespace, Name: objectKey.Name},
		}

		quota = &gardencorev1beta1.Quota{
			ObjectMeta: metav1.ObjectMeta{Namespace: objectKey.Namespace, Name: objectKey.Name},
			Spec: gardencorev1beta1.QuotaSpec{
				Scope: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
				},
			},
		}

		secretBinding = &gardencorev1beta1.SecretBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: objectKey.Namespace, Name: objectKey.Name},
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

		credentialsBinding = &securityv1alpha1.CredentialsBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: objectKey.Namespace, Name: objectKey.Name},
			Provider: securityv1alpha1.CredentialsBindingProvider{
				Type: providerType,
			},
			CredentialsRef: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       resourceName,
				Namespace:  testNamespace.Name,
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

		if credentialsBinding != nil {
			By("Create CredentialsBinding")
			Expect(testClient.Create(ctx, credentialsBinding)).To(Succeed())
			log.Info("Created CredentialsBinding for test", "credentialsbinding", client.ObjectKeyFromObject(credentialsBinding))

			By("Wait until manager has observed CredentialsBinding")
			// Use the manager's cache to ensure it has observed the CredentialsBinding.
			// Otherwise, the controller might clean up the Quota too early because it thinks all referencing
			// CredentialsBinding are gone. Similar to https://github.com/gardener/gardener/issues/6486 and
			// https://github.com/gardener/gardener/issues/6607.
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(credentialsBinding), &securityv1alpha1.CredentialsBinding{})
			}).Should(Succeed())

			DeferCleanup(func() {
				By("Delete CredentialsBinding")
				Expect(testClient.Delete(ctx, credentialsBinding)).To(Or(Succeed(), BeNotFoundError()))
			})
		}
	})

	Context("no SecretBinding & CredentialsBinding referencing Quota", func() {
		BeforeEach(func() {
			secretBinding = nil
			credentialsBinding = nil
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
		BeforeEach(func() {
			credentialsBinding = nil
		})

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

	Context("CredentialsBinding referencing Quota", func() {
		BeforeEach(func() {
			secretBinding = nil
		})

		JustBeforeEach(func() {
			By("Ensure finalizer got added")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, objectKey, quota)).To(Succeed())
				g.Expect(quota.Finalizers).To(ConsistOf("gardener"))
			}).Should(Succeed())

			By("Delete Quota")
			Expect(testClient.Delete(ctx, quota)).To(Succeed())
		})

		It("should add finalizer and not release it on deletion since there is still referencing CredentialsBinding", func() {
			By("Ensure Quota is not released")
			Consistently(func() error {
				return testClient.Get(ctx, objectKey, quota)
			}).Should(Succeed())
		})

		It("should add the finalizer and release it on deletion after CredentialsBinding got deleted", func() {
			By("Delete CredentialsBinding")
			Expect(testClient.Delete(ctx, credentialsBinding)).To(Succeed())

			By("Ensure Quota is released")
			Eventually(func() error {
				return testClient.Get(ctx, objectKey, quota)
			}).Should(BeNotFoundError())
		})
	})
})
