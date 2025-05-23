// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package credentialsbinding_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CredentialsBinding controller test", func() {
	// TODO(dimityrmirchev): This test suite should eventually handle references to workload identity
	var (
		providerType = "provider"

		secret             *corev1.Secret
		quota              *gardencorev1beta1.Quota
		credentialsBinding *securityv1alpha1.CredentialsBinding
		shoot              *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.ShootCredentialsBinding, true))
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

		credentialsBinding = &securityv1alpha1.CredentialsBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testID + "-" + utils.ComputeSHA256Hex([]byte(testNamespace.Name + CurrentSpecReport().LeafNodeLocation.String()))[:8],
				Namespace: testNamespace.Name,
				Labels:    map[string]string{testID: testRunID},
			},
			Provider: securityv1alpha1.CredentialsBindingProvider{
				Type: providerType,
			},
			CredentialsRef: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       secret.Name,
				Namespace:  secret.Namespace,
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
				CloudProfileName:       ptr.To("test-cloudprofile"),
				CredentialsBindingName: ptr.To(credentialsBinding.Name),
				Region:                 "foo-region",
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
			// Otherwise, the controller might clean up the CredentialsBinding too early because it thinks all referencing shoots
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

		By("Create CredentialsBinding")
		Expect(testClient.Create(ctx, credentialsBinding)).To(Succeed())
		log.Info("Created CredentialsBinding for test", "credentialsBinding", client.ObjectKeyFromObject(credentialsBinding))

		DeferCleanup(func() {
			if shoot != nil {
				// delete the shoot first, otherwise CredentialsBinding will not be released
				By("Delete Shoot")
				Expect(testClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))
			}

			By("Delete CredentialsBinding")
			Expect(testClient.Delete(ctx, credentialsBinding)).To(Or(Succeed(), BeNotFoundError()))
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(credentialsBinding), credentialsBinding)
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

	Context("no shoot referencing the CredentialsBinding", func() {
		BeforeEach(func() {
			shoot = nil
		})

		It("should add the finalizer and release it on deletion", func() {
			By("Ensure finalizer got added to CredentialsBinding")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(credentialsBinding), credentialsBinding)).To(Succeed())
				g.Expect(credentialsBinding.Finalizers).To(ConsistOf("gardener"))
			}).Should(Succeed())

			By("Ensure finalizer and labels got added to Secret and Quota")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				g.Expect(secret.Finalizers).To(ConsistOf("gardener.cloud/gardener"))
				g.Expect(secret.Labels).To(And(
					HaveKeyWithValue("provider.shoot.gardener.cloud/"+providerType, "true"),
					HaveKeyWithValue("reference.gardener.cloud/credentialsbinding", "true"),
				))
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(quota), quota)).To(Succeed())
				g.Expect(quota.Labels).To(HaveKeyWithValue("reference.gardener.cloud/credentialsbinding", "true"))
			}).Should(Succeed())

			By("Delete CredentialsBinding")
			Expect(testClient.Delete(ctx, credentialsBinding)).To(Succeed())

			By("Ensure CredentialsBinding is released")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(credentialsBinding), credentialsBinding)
			}).Should(BeNotFoundError())

			By("Ensure finalizer and labels got removed from Secret and Quota")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				g.Expect(secret.Finalizers).NotTo(ContainElement("gardener.cloud/gardener"))
				g.Expect(secret.Labels).NotTo(HaveKeyWithValue("reference.gardener.cloud/credentialsbinding", "true"))
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(quota), quota)).To(Succeed())
				g.Expect(quota.Labels).NotTo(HaveKeyWithValue("reference.gardener.cloud/credentialsbinding", "true"))
			}).Should(Succeed())
		})

		It("should not remove finalizer and labels from Secret and Quota because other CredentialsBinding still references them", func() {
			By("Ensure finalizer and labels got added to Secret and Quota")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				g.Expect(secret.Finalizers).To(ConsistOf("gardener.cloud/gardener"))
				g.Expect(secret.Labels).To(And(
					HaveKeyWithValue("provider.shoot.gardener.cloud/"+providerType, "true"),
					HaveKeyWithValue("reference.gardener.cloud/credentialsbinding", "true"),
				))
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(quota), quota)).To(Succeed())
				g.Expect(quota.Labels).To(HaveKeyWithValue("reference.gardener.cloud/credentialsbinding", "true"))
			}).Should(Succeed())

			By("Create second CredentialsBinding")
			credentialsBinding2 := credentialsBinding.DeepCopy()
			credentialsBinding2.ObjectMeta = metav1.ObjectMeta{
				GenerateName: "credentialsbinding-",
				Namespace:    testNamespace.Name,
			}
			Expect(testClient.Create(ctx, credentialsBinding2)).To(Succeed())

			By("Delete first CredentialsBinding")
			Expect(testClient.Delete(ctx, credentialsBinding)).To(Succeed())

			By("Ensure first CredentialsBinding is released")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(credentialsBinding), credentialsBinding)
			}).Should(BeNotFoundError())

			By("Ensure finalizer and labels are still present on Secret and Quota")
			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				g.Expect(secret.Finalizers).To(ConsistOf("gardener.cloud/gardener"))
				g.Expect(secret.Labels).To(And(
					HaveKeyWithValue("provider.shoot.gardener.cloud/"+providerType, "true"),
					HaveKeyWithValue("reference.gardener.cloud/credentialsbinding", "true"),
				))
			}).Should(Succeed())

			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(quota), quota)).To(Succeed())
				g.Expect(quota.Labels).To(HaveKeyWithValue("reference.gardener.cloud/credentialsbinding", "true"))
			}).Should(Succeed())

			By("Delete second CredentialsBinding")
			Expect(testClient.Delete(ctx, credentialsBinding2)).To(Succeed())

			By("Ensure second CredentialsBinding is released")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(credentialsBinding2), credentialsBinding2)
			}).Should(BeNotFoundError())
		})
	})

	Context("shoots referencing the CredentialsBinding", func() {
		JustBeforeEach(func() {
			By("Ensure finalizer got added")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(credentialsBinding), credentialsBinding)).To(Succeed())
				g.Expect(credentialsBinding.Finalizers).To(ConsistOf("gardener"))
			}).Should(Succeed())

			By("Delete CredentialsBinding")
			Expect(testClient.Delete(ctx, credentialsBinding)).To(Succeed())
		})

		It("should add the finalizer and not release it on deletion since there is still referencing shoot", func() {
			By("Ensure CredentialsBinding is not released")
			Consistently(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(credentialsBinding), credentialsBinding)
			}).Should(Succeed())
		})

		It("should add the finalizer and release it on deletion after the shoot got deleted", func() {
			By("Delete Shoot")
			Expect(testClient.Delete(ctx, shoot)).To(Succeed())

			By("Ensure CredentialsBinding is released")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(credentialsBinding), credentialsBinding)
			}).Should(BeNotFoundError())
		})
	})
})
