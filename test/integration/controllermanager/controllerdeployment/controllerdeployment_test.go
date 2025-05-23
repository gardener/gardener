// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerdeployment_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ControllerDeployment controller tests", func() {
	var (
		controllerDeployment   *gardencorev1.ControllerDeployment
		controllerRegistration *gardencorev1beta1.ControllerRegistration
	)

	BeforeEach(func() {
		controllerDeployment = &gardencorev1.ControllerDeployment{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
				Labels:       map[string]string{testID: testRunID},
			},
			Helm: &gardencorev1.HelmControllerDeployment{
				RawChart: []byte("foo"),
			},
		}

		controllerRegistration = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
				Labels:       map[string]string{testID: testRunID},
			},
		}
	})

	JustBeforeEach(func() {
		By("Create ControllerDeployment")
		Expect(testClient.Create(ctx, controllerDeployment)).To(Succeed())
		log.Info("Created ControllerDeployment for test", "controllerDeployment", client.ObjectKeyFromObject(controllerDeployment))

		DeferCleanup(func() {
			By("Delete ControllerDeployment")
			Expect(testClient.Delete(ctx, controllerDeployment)).To(Or(Succeed(), BeNotFoundError()))
		})

		if controllerRegistration != nil {
			controllerRegistration.Spec = gardencorev1beta1.ControllerRegistrationSpec{
				Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
					DeploymentRefs: []gardencorev1beta1.DeploymentRef{
						{Name: controllerDeployment.Name},
					},
				},
			}
			By("Create ControllerRegistration")
			Expect(testClient.Create(ctx, controllerRegistration)).To(Succeed())
			log.Info("Created ControllerRegistration for test", "controllerRegistration", client.ObjectKeyFromObject(controllerRegistration))

			By("Wait until manager has observed ControllerRegistration")
			// Use the manager's cache to ensure it has observed the ControllerRegistration.
			// Otherwise, the controller might clean up the ControllerDeployment too early because it thinks all referencing ControllerRegistrations
			// are gone. Similar to https://github.com/gardener/gardener/issues/6486
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), &gardencorev1beta1.ControllerRegistration{})
			}).Should(Succeed())

			DeferCleanup(func() {
				By("Delete ControllerRegistration")
				Expect(testClient.Delete(ctx, controllerRegistration)).To(Or(Succeed(), BeNotFoundError()))
			})
		}
	})

	Context("no ControllerRegistration referencing the ControllerDeployment", func() {
		BeforeEach(func() {
			controllerRegistration = nil
		})

		It("should add the finalizer and release it on deletion", func() {
			By("Ensure finalizer got added")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerDeployment), controllerDeployment)).To(Succeed())
				g.Expect(controllerDeployment.Finalizers).To(ConsistOf(FinalizerName))
			}).Should(Succeed())

			By("Delete ControllerDeployment")
			Expect(testClient.Delete(ctx, controllerDeployment)).To(Succeed())

			By("Ensure ControllerDeployment is released")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(controllerDeployment), controllerDeployment)
			}).Should(BeNotFoundError())
		})
	})

	Context("controllerRegistration referencing controllerDeployment", func() {
		JustBeforeEach(func() {
			By("Ensure finalizer got added")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerDeployment), controllerDeployment)).To(Succeed())
				g.Expect(controllerDeployment.Finalizers).To(ConsistOf(FinalizerName))
			}).Should(Succeed())

			By("Delete ControllerDeployment")
			Expect(testClient.Delete(ctx, controllerDeployment)).To(Succeed())
		})

		It("should add the finalizer and not release it on deletion since there still is a referencing controllerRegistration", func() {
			By("Ensure ControllerDeployment is not released")
			Consistently(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(controllerDeployment), controllerDeployment)
			}).Should(Succeed())
		})

		It("should add the finalizer and release it on deletion after the ControllerRegistration got deleted", func() {
			By("Delete ControllerRegistration")
			Expect(testClient.Delete(ctx, controllerRegistration)).To(Succeed())

			By("Ensure ControllerDeployment is released")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(controllerDeployment), controllerDeployment)
			}).Should(BeNotFoundError())
		})
	})
})
