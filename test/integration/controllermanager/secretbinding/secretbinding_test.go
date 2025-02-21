// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretbinding_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("SecretBinding controller test", func() {
	var (
		providerType = "provider"

		secret        *corev1.Secret
		quota         *gardencorev1beta1.Quota
		secretBinding *gardencorev1beta1.SecretBinding
		shoot         *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testID + "-" + utils.ComputeSHA256Hex([]byte(testNamespace.Name + CurrentSpecReport().LeafNodeLocation.String()))[:8],
				Namespace: testNamespace.Name,
				Labels:    map[string]string{testID: testRunID},
			},
		}

		quota = &gardencorev1beta1.Quota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testID + "-" + utils.ComputeSHA256Hex([]byte(testNamespace.Name + CurrentSpecReport().LeafNodeLocation.String()))[:8],
				Namespace: testNamespace.Name,
				Labels:    map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.QuotaSpec{
				Scope: corev1.ObjectReference{
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Kind:       "Project",
				},
			},
		}

		secretBinding = &gardencorev1beta1.SecretBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testID + "-" + utils.ComputeSHA256Hex([]byte(testNamespace.Name + CurrentSpecReport().LeafNodeLocation.String()))[:8],
				Namespace: testNamespace.Name,
				Labels:    map[string]string{testID: testRunID},
			},
			Provider: &gardencorev1beta1.SecretBindingProvider{
				Type: providerType,
			},
			SecretRef: corev1.SecretReference{
				Name:      secret.Name,
				Namespace: secret.Namespace,
			},
			Quotas: []corev1.ObjectReference{{
				Name:      quota.Name,
				Namespace: quota.Namespace,
			}},
		}

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
			},
			Spec: gardencorev1beta1.ShootSpec{
				CloudProfileName:  ptr.To("test-cloudprofile"),
				SecretBindingName: ptr.To(secretBinding.Name),
				Region:            "foo-region",
				Provider: gardencorev1beta1.Provider{
					Type: "test-provider",
					Workers: []gardencorev1beta1.Worker{
						{
							Name:    "cpu-worker",
							Minimum: 2,
							Maximum: 2,
							Machine: gardencorev1beta1.Machine{Type: "large"},
						},
					},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{Version: "1.31.1"},
				Networking: &gardencorev1beta1.Networking{Type: ptr.To("foo-networking")},
			},
		}
	})

	JustBeforeEach(func() {
		if shoot != nil {
			// Create the shoot first and wait until the manager's cache has observed it.
			// Otherwise, the controller might clean up the SecretBinding too early because it thinks all referencing shoots
			// are gone. Similar to https://github.com/gardener/gardener/issues/6486
			By("Create Shoot")
			Expect(testClient.Create(ctx, shoot)).To(Succeed())
			log.Info("Created Shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

			By("Wait until manager has observed shoot")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), &gardencorev1beta1.Shoot{})
			}).Should(Succeed())
		}

		By("Create Secret")
		Expect(testClient.Create(ctx, secret)).To(Succeed())
		log.Info("Created Secret for test", "secret", client.ObjectKeyFromObject(secret))

		By("Create Quota")
		Expect(testClient.Create(ctx, quota)).To(Succeed())
		log.Info("Created Quota for test", "quota", client.ObjectKeyFromObject(quota))

		By("Create SecretBinding")
		Expect(testClient.Create(ctx, secretBinding)).To(Succeed())
		log.Info("Created SecretBinding for test", "secretBinding", client.ObjectKeyFromObject(secretBinding))

		DeferCleanup(func() {
			if shoot != nil {
				// delete the shoot first, otherwise SecretBinding will not be released
				By("Delete Shoot")
				Expect(testClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))
			}

			By("Delete SecretBinding")
			Expect(testClient.Delete(ctx, secretBinding)).To(Or(Succeed(), BeNotFoundError()))
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(secretBinding), secretBinding)
			}).Should(BeNotFoundError())

			By("Delete Secret")
			Expect(testClient.Delete(ctx, secret)).To(Or(Succeed(), BeNotFoundError()))
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)
			}).Should(BeNotFoundError())

			By("Delete Quota")
			Expect(testClient.Delete(ctx, quota)).To(Or(Succeed(), BeNotFoundError()))
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(quota), quota)
			}).Should(BeNotFoundError())
		})
	})

	Context("no shoot referencing the SecretBinding", func() {
		BeforeEach(func() {
			shoot = nil
		})

		It("should add the finalizer and release it on deletion", func() {
			By("Ensure finalizer got added to SecretBinding")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretBinding), secretBinding)).To(Succeed())
				g.Expect(secretBinding.Finalizers).To(ConsistOf("gardener"))
			}).Should(Succeed())

			By("Ensure finalizer and labels got added to Secret and Quota")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				g.Expect(secret.Finalizers).To(ConsistOf("gardener.cloud/gardener"))
				g.Expect(secret.Labels).To(And(
					HaveKeyWithValue("provider.shoot.gardener.cloud/"+providerType, "true"),
					HaveKeyWithValue("reference.gardener.cloud/secretbinding", "true"),
				))
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(quota), quota)).To(Succeed())
				g.Expect(quota.Labels).To(HaveKeyWithValue("reference.gardener.cloud/secretbinding", "true"))
			}).Should(Succeed())

			By("Delete SecretBinding")
			Expect(testClient.Delete(ctx, secretBinding)).To(Succeed())

			By("Ensure SecretBinding is released")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(secretBinding), secretBinding)
			}).Should(BeNotFoundError())

			By("Ensure finalizer and labels got removed from Secret and Quota")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				g.Expect(secret.Finalizers).NotTo(ContainElement("gardener.cloud/gardener"))
				g.Expect(secret.Labels).NotTo(HaveKeyWithValue("reference.gardener.cloud/secretbinding", "true"))
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(quota), quota)).To(Succeed())
				g.Expect(quota.Labels).NotTo(HaveKeyWithValue("reference.gardener.cloud/secretbinding", "true"))
			}).Should(Succeed())
		})

		It("should not remove finalizer and labels from Secret and Quota because other SecretBinding still references them", func() {
			By("Ensure finalizer and labels got added to Secret and Quota")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				g.Expect(secret.Finalizers).To(ConsistOf("gardener.cloud/gardener"))
				g.Expect(secret.Labels).To(And(
					HaveKeyWithValue("provider.shoot.gardener.cloud/"+providerType, "true"),
					HaveKeyWithValue("reference.gardener.cloud/secretbinding", "true"),
				))
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(quota), quota)).To(Succeed())
				g.Expect(quota.Labels).To(HaveKeyWithValue("reference.gardener.cloud/secretbinding", "true"))
			}).Should(Succeed())

			By("Create second SecretBinding")
			secretBinding2 := secretBinding.DeepCopy()
			secretBinding2.ObjectMeta = metav1.ObjectMeta{
				GenerateName: "secretbinding2-",
				Namespace:    testNamespace.Name,
			}
			Expect(testClient.Create(ctx, secretBinding2)).To(Succeed())

			By("Delete first SecretBinding")
			Expect(testClient.Delete(ctx, secretBinding)).To(Succeed())

			By("Ensure first SecretBinding is released")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(secretBinding), secretBinding)
			}).Should(BeNotFoundError())

			By("Ensure finalizer and labels are still present on Secret and Quota")
			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				g.Expect(secret.Finalizers).To(ConsistOf("gardener.cloud/gardener"))
				g.Expect(secret.Labels).To(And(
					HaveKeyWithValue("provider.shoot.gardener.cloud/"+providerType, "true"),
					HaveKeyWithValue("reference.gardener.cloud/secretbinding", "true"),
				))
			}).Should(Succeed())

			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(quota), quota)).To(Succeed())
				g.Expect(quota.Labels).To(HaveKeyWithValue("reference.gardener.cloud/secretbinding", "true"))
			}).Should(Succeed())

			By("Delete second SecretBinding")
			Expect(testClient.Delete(ctx, secretBinding2)).To(Succeed())

			By("Ensure second SecretBinding is released")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(secretBinding2), secretBinding2)
			}).Should(BeNotFoundError())
		})
	})

	Context("shoots referencing the SecretBinding", func() {
		JustBeforeEach(func() {
			By("Ensure finalizer got added")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretBinding), secretBinding)).To(Succeed())
				g.Expect(secretBinding.Finalizers).To(ConsistOf("gardener"))
			}).Should(Succeed())

			By("Delete SecretBinding")
			Expect(testClient.Delete(ctx, secretBinding)).To(Succeed())
		})

		It("should add the finalizer and not release it on deletion since there is still referencing shoot", func() {
			By("Ensure SecretBinding is not released")
			Consistently(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(secretBinding), secretBinding)
			}).Should(Succeed())
		})

		It("should add the finalizer and release it on deletion after the shoot got deleted", func() {
			By("Delete Shoot")
			Expect(testClient.Delete(ctx, shoot)).To(Succeed())

			By("Ensure SecretBinding is released")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(secretBinding), secretBinding)
			}).Should(BeNotFoundError())
		})
	})
})
