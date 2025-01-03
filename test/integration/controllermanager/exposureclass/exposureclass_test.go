// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package exposureclass_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ExposureClass controller test", func() {
	var (
		exposureClass *gardencorev1beta1.ExposureClass
		shoot         *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		exposureClass = &gardencorev1beta1.ExposureClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:   testID + "-" + utils.ComputeSHA256Hex([]byte(testNamespace.Name + CurrentSpecReport().LeafNodeLocation.String()))[:8],
				Labels: map[string]string{testID: testRunID},
			},
			Handler: "test-exposure-class-handler-name",
			Scheduling: &gardencorev1beta1.ExposureClassScheduling{
				SeedSelector: &gardencorev1beta1.SeedSelector{
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test": "foo",
						},
					},
				},
			},
		}

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
			},
			Spec: gardencorev1beta1.ShootSpec{
				ExposureClassName: ptr.To(exposureClass.Name),
				CloudProfileName:  ptr.To("test-cloudprofile"),
				SecretBindingName: ptr.To("my-provider-account"),
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
			// Otherwise, the controller might clean up the ExposureClass too early because it thinks all referencing shoots
			// are gone. Similar to https://github.com/gardener/gardener/issues/6486
			By("Create Shoot")
			Expect(testClient.Create(ctx, shoot)).To(Succeed())
			log.Info("Created Shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

			By("Wait until manager has observed shoot")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), &gardencorev1beta1.Shoot{})
			}).Should(Succeed())
		}

		By("Create ExposureClass")
		Expect(testClient.Create(ctx, exposureClass)).To(Succeed())
		log.Info("Created ExposureClass for test", "exposureClass", client.ObjectKeyFromObject(exposureClass))

		DeferCleanup(func() {
			if shoot != nil {
				// delete the shoot first, otherwise exposureclass will not be released
				By("Delete Shoot")
				Expect(testClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))
			}

			By("Delete ExposureClass")
			Expect(testClient.Delete(ctx, exposureClass)).To(Or(Succeed(), BeNotFoundError()))
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(exposureClass), exposureClass)
			}).Should(BeNotFoundError())
		})
	})

	Context("no shoot referencing the ExposureClass", func() {
		BeforeEach(func() {
			shoot = nil
		})

		It("should add the finalizer and release it on deletion", func() {
			By("Ensure finalizer got added")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(exposureClass), exposureClass)).To(Succeed())
				g.Expect(exposureClass.Finalizers).To(ConsistOf("gardener"))
			}).Should(Succeed())

			By("Delete ExposureClass")
			Expect(testClient.Delete(ctx, exposureClass)).To(Succeed())

			By("Ensure ExposureClass is released")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(exposureClass), exposureClass)
			}).Should(BeNotFoundError())
		})
	})

	Context("shoots referencing the ExposureClass", func() {
		JustBeforeEach(func() {
			By("Ensure finalizer got added")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(exposureClass), exposureClass)).To(Succeed())
				g.Expect(exposureClass.Finalizers).To(ConsistOf("gardener"))
			}).Should(Succeed())

			By("Delete ExposureClass")
			Expect(testClient.Delete(ctx, exposureClass)).To(Succeed())
		})

		It("should add the finalizer and not release it on deletion since there is still referencing shoot", func() {
			By("Ensure ExposureClass is not released")
			Consistently(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(exposureClass), exposureClass)
			}).Should(Succeed())
		})

		It("should add the finalizer and release it on deletion after the shoot got deleted", func() {
			By("Delete Shoot")
			Expect(testClient.Delete(ctx, shoot)).To(Succeed())

			By("Ensure ExposureClass is released")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(exposureClass), exposureClass)
			}).Should(BeNotFoundError())
		})
	})
})
