// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package secret_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

const finalizerName = "resources.gardener.cloud/gardener-resource-manager"

var _ = Describe("Secret controller tests", func() {
	var (
		secretFoo *corev1.Secret
		secretBar *corev1.Secret
	)

	BeforeEach(func() {

		secretFoo = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "foo-",
				Namespace:    testNamespace.Name,
				Finalizers:   []string{"resources.gardener.cloud/gardener-resource-manager"},
			},
		}

		secretBar = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "bar-",
				Namespace:    testNamespace.Name,
			},
		}
	})

	JustBeforeEach(func() {
		By("Create secret for test")
		Expect(testClient.Create(ctx, secretFoo)).To(Succeed())
		log.Info("Created Secret for test", "secretName", secretFoo.Name)
		Expect(testClient.Create(ctx, secretBar)).To(Succeed())
		log.Info("Created Secret for test", "secretName", secretBar.Name)
	})

	AfterEach(func() {
		Expect(testClient.Delete(ctx, secretFoo)).To(Or(Succeed(), BeNotFoundError()))
		Expect(testClient.Delete(ctx, secretBar)).To(Or(Succeed(), BeNotFoundError()))
		// Wait for clean up of the secret
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(secretFoo), secretFoo)
		}).Should(BeNotFoundError())
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(secretBar), secretBar)
		}).Should(BeNotFoundError())
	})

	Context("Secret finalizer", func() {

		It("should remove finalizer from secret if finalizer is present", func() {

			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretFoo), secretFoo)).To(Succeed())
				return secretFoo.ObjectMeta.Finalizers
			}).ShouldNot(
				ContainElement(finalizerName),
			)
		})

		It("should do nothing if secret has no finalizer", func() {

			Consistently(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretBar), secretBar)).To(Succeed())
				return secretBar.ObjectMeta.Finalizers
			}).ShouldNot(
				ContainElement(finalizerName),
			)
		})
	})
})
