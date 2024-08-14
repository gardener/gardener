// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package referencecleaner_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CredentialsBinding controller test", func() {
	var (
		providerType = "provider"

		secret              *corev1.Secret
		workloadIdentity    *securityv1alpha1.WorkloadIdentity
		credentialsBinding1 *securityv1alpha1.CredentialsBinding
		credentialsBinding2 *securityv1alpha1.CredentialsBinding
	)

	BeforeEach(func() {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:       testID + "-" + utils.ComputeSHA256Hex([]byte(testNamespace.Name + CurrentSpecReport().LeafNodeLocation.String()))[:8],
				Finalizers: []string{"gardener.cloud/gardener"},
				Namespace:  testNamespace.Name,
				Labels: map[string]string{
					testID: testRunID,
					"provider.shoot.gardener.cloud/" + providerType: "true",
					"reference.gardener.cloud/credentialsbinding":   "true",
				},
			},
		}

		workloadIdentity = &securityv1alpha1.WorkloadIdentity{
			ObjectMeta: metav1.ObjectMeta{
				Name:       testID + "-" + utils.ComputeSHA256Hex([]byte(testNamespace.Name + CurrentSpecReport().LeafNodeLocation.String()))[:8],
				Finalizers: []string{"gardener.cloud/gardener"},
				Namespace:  testNamespace.Name,
				Labels: map[string]string{
					testID: testRunID,
					"provider.shoot.gardener.cloud/" + providerType: "true",
					"reference.gardener.cloud/credentialsbinding":   "true",
				},
			},
			Spec: securityv1alpha1.WorkloadIdentitySpec{
				Audiences: []string{"foo"},
				TargetSystem: securityv1alpha1.TargetSystem{
					Type: providerType,
				},
			},
		}

		credentialsBinding1 = &securityv1alpha1.CredentialsBinding{
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
		}

		credentialsBinding2 = &securityv1alpha1.CredentialsBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testID + "-" + utils.ComputeSHA256Hex([]byte(testNamespace.Name + CurrentSpecReport().LeafNodeLocation.String()))[:8],
				Namespace: testNamespace.Name,
				Labels:    map[string]string{testID: testRunID},
			},
			Provider: securityv1alpha1.CredentialsBindingProvider{
				Type: providerType,
			},
			CredentialsRef: corev1.ObjectReference{
				APIVersion: securityv1alpha1.SchemeGroupVersion.String(),
				Kind:       "WorkloadIdentity",
				Name:       workloadIdentity.Name,
				Namespace:  workloadIdentity.Namespace,
			},
		}
	})

	Context("Credentials of type Secret", func() {
		JustBeforeEach(func() {
			// Create the credentialsbinding first and wait until the manager's cache has observed it.
			// Otherwise, the controller might clean up the Secret/WorkloadIdentities too early because it thinks all referencing CredentialsBindings
			// are gone. Similar to https://github.com/gardener/gardener/issues/6486
			By("Create CredentialsBinding")
			Expect(testClient.Create(ctx, credentialsBinding1)).To(Succeed())
			log.Info("Created CredentialsBinding for test", "credentialsBinding", client.ObjectKeyFromObject(credentialsBinding1))

			By("Wait until manager has observed the CredentialsBinding")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(credentialsBinding1), &securityv1alpha1.CredentialsBinding{})
			}).Should(Succeed())

			By("Create Secret")
			Expect(testClient.Create(ctx, secret)).To(Succeed())
			log.Info("Created Secret for test", "secret", client.ObjectKeyFromObject(secret))

			DeferCleanup(func() {
				By("Delete CredentialsBinding")
				Expect(testClient.Delete(ctx, credentialsBinding1)).To(Or(Succeed(), BeNotFoundError()))
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(credentialsBinding1), credentialsBinding1)
				}).Should(BeNotFoundError())

				By("Delete Secret")
				secret.Finalizers = []string{}
				Expect(testClient.Update(ctx, secret)).To(Or(Succeed(), BeNotFoundError()))
				Expect(testClient.Delete(ctx, secret)).To(Or(Succeed(), BeNotFoundError()))
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)
				}).Should(BeNotFoundError())
			})
		})

		Context("no CredentialsBinding referencing the Secret", func() {
			It("should release the Secret", func() {
				By("Ensure that Secret is not being deleted")
				Consistently(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				}).Should(Succeed())

				By("Delete the CredentialsBinding")
				Expect(testClient.Delete(ctx, credentialsBinding1)).To(Succeed())

				By("Ensure finalizer and labels are removed from Secret")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
					g.Expect(secret.Finalizers).To(BeEmpty())
					g.Expect(secret.Labels).ToNot(And(
						HaveKey("provider.shoot.gardener.cloud/"+providerType),
						HaveKey("reference.gardener.cloud/credentialsbinding"),
					))
				}).Should(Succeed())
			})

			It("should release the Secret but keep finalizer and provider type label because of SecretBinding ref label", func() {
				secret.Labels["reference.gardener.cloud/secretbinding"] = "true"
				Expect(testClient.Update(ctx, secret)).To(Succeed())

				By("Ensure that secret is not being deleted")
				Consistently(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				}).Should(Succeed())

				By("Delete the CredentialsBinding")
				Expect(testClient.Delete(ctx, credentialsBinding1)).To(Succeed())

				By("Ensure finalizer and labels are removed from Secret")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
					g.Expect(secret.Finalizers).To(ConsistOf("gardener.cloud/gardener"))
					g.Expect(secret.Labels).ToNot(And(
						HaveKey("reference.gardener.cloud/credentialsbinding"),
					))
					g.Expect(secret.Labels).To(And(
						HaveKeyWithValue("provider.shoot.gardener.cloud/"+providerType, "true"),
						HaveKeyWithValue("reference.gardener.cloud/secretbinding", "true"),
					))
				}).Should(Succeed())
			})
		})

		Context("CredentialsBinding referencing the Secret", func() {
			It("should not release the secret", func() {
				By("Ensure that secret is not being deleted")
				Consistently(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				}).Should(Succeed())
			})

			It("should not release the secret because it is referenced from a CredentialsBinding in another namespace", func() {
				anotherCredentialsBinding := &securityv1alpha1.CredentialsBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testID + "-" + utils.ComputeSHA256Hex([]byte(testNamespace.Name + CurrentSpecReport().LeafNodeLocation.String()))[:8],
						Namespace: anotherNamespace.Name,
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
				}
				By("Create new CredentialsBinding")
				Expect(testClient.Create(ctx, anotherCredentialsBinding)).To(Succeed())
				By("Wait until manager has observed the CredentialsBinding")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(anotherCredentialsBinding), &securityv1alpha1.CredentialsBinding{})
				}).Should(Succeed())

				By("Delete the original CredentialsBinding")
				Expect(testClient.Delete(ctx, credentialsBinding1)).To(Succeed())

				By("Ensure that Secret is not being deleted")
				Consistently(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				}).Should(Succeed())

				Expect(testClient.Delete(ctx, anotherCredentialsBinding)).To(Succeed())
			})
		})
	})

	Context("Credentials of type WorkloadIdentity", func() {
		JustBeforeEach(func() {
			// Create the credentialsbinding first and wait until the manager's cache has observed it.
			// Otherwise, the controller might clean up the Secret/WorkloadIdentities too early because it thinks all referencing CredentialsBindings
			// are gone. Similar to https://github.com/gardener/gardener/issues/6486
			By("Create CredentialsBinding")
			Expect(testClient.Create(ctx, credentialsBinding2)).To(Succeed())
			log.Info("Created CredentialsBinding for test", "credentialsBinding", client.ObjectKeyFromObject(credentialsBinding2))

			By("Wait until manager has observed the CredentialsBinding")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(credentialsBinding2), &securityv1alpha1.CredentialsBinding{})
			}).Should(Succeed())

			By("Create WorkloadIdentity")
			Expect(testClient.Create(ctx, workloadIdentity)).To(Succeed())
			log.Info("Created WorkloadIdentity for test", "workloadIdentity", client.ObjectKeyFromObject(workloadIdentity))

			DeferCleanup(func() {
				By("Delete CredentialsBinding")
				Expect(testClient.Delete(ctx, credentialsBinding2)).To(Or(Succeed(), BeNotFoundError()))
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(credentialsBinding2), credentialsBinding2)
				}).Should(BeNotFoundError())

				By("Delete WorkloadIdentity")
				workloadIdentity.Finalizers = []string{}
				Expect(testClient.Update(ctx, workloadIdentity)).To(Or(Succeed(), BeNotFoundError()))
				Expect(testClient.Delete(ctx, workloadIdentity)).To(Or(Succeed(), BeNotFoundError()))
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(workloadIdentity), workloadIdentity)
				}).Should(BeNotFoundError())
			})
		})

		Context("no CredentialsBinding referencing the WorkloadIdentity", func() {
			It("should release the WorkloadIdentity", func() {
				By("Ensure that WorkloadIdentity is not being deleted")
				Consistently(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(workloadIdentity), workloadIdentity)).To(Succeed())
				}).Should(Succeed())

				By("Delete the CredentialsBinding")
				Expect(testClient.Delete(ctx, credentialsBinding2)).To(Succeed())

				By("Ensure finalizer and labels are removed from WorkloadIdentity")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(workloadIdentity), workloadIdentity)).To(Succeed())
					g.Expect(workloadIdentity.Finalizers).To(BeEmpty())
					g.Expect(workloadIdentity.Labels).ToNot(And(
						HaveKey("provider.shoot.gardener.cloud/"+providerType),
						HaveKey("reference.gardener.cloud/credentialsbinding"),
					))
				}).Should(Succeed())
			})
		})

		Context("CredentialsBinding referencing the WorkloadIdentity", func() {
			It("should not release the WorkloadIdentity", func() {
				By("Ensure that WorkloadIdentity is not being deleted")
				Consistently(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(workloadIdentity), workloadIdentity)).To(Succeed())
				}).Should(Succeed())
			})

			It("should not release the secret because it is referenced from a CredentialsBinding in another namespace", func() {
				anotherCredentialsBinding := &securityv1alpha1.CredentialsBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testID + "-" + utils.ComputeSHA256Hex([]byte(testNamespace.Name + CurrentSpecReport().LeafNodeLocation.String()))[:8],
						Namespace: anotherNamespace.Name,
						Labels:    map[string]string{testID: testRunID},
					},
					Provider: securityv1alpha1.CredentialsBindingProvider{
						Type: providerType,
					},
					CredentialsRef: corev1.ObjectReference{
						APIVersion: securityv1alpha1.SchemeGroupVersion.String(),
						Kind:       "WorkloadIdentity",
						Name:       workloadIdentity.Name,
						Namespace:  workloadIdentity.Namespace,
					},
				}
				By("Create new CredentialsBinding")
				Expect(testClient.Create(ctx, anotherCredentialsBinding)).To(Succeed())
				By("Wait until manager has observed the CredentialsBinding")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(anotherCredentialsBinding), &securityv1alpha1.CredentialsBinding{})
				}).Should(Succeed())

				By("Delete the original CredentialsBinding")
				Expect(testClient.Delete(ctx, credentialsBinding2)).To(Succeed())

				By("Ensure that WorkloadIdentity is not being deleted")
				Consistently(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(workloadIdentity), workloadIdentity)).To(Succeed())
				}).Should(Succeed())

				Expect(testClient.Delete(ctx, anotherCredentialsBinding)).To(Succeed())
			})
		})
	})
})
