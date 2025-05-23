// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ControllerInstallation Required controller tests", func() {
	var (
		extensionType = "type1"

		controllerRegistration *gardencorev1beta1.ControllerRegistration
		controllerInstallation *gardencorev1beta1.ControllerInstallation
		infrastructure         *extensionsv1alpha1.Infrastructure
	)

	BeforeEach(func() {
		By("Create ControllerRegistration")
		controllerRegistration = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "ctrlreg1-",
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{Kind: extensionsv1alpha1.InfrastructureResource, Type: extensionType},
					{Kind: extensionsv1alpha1.ControlPlaneResource, Type: "foo"},
				},
			},
		}

		Expect(testClient.Create(ctx, controllerRegistration)).To(Succeed())
		log.Info("Created ControllerRegistration for test", "controllerRegistration", client.ObjectKeyFromObject(controllerRegistration))

		DeferCleanup(func() {
			By("Delete ControllerRegistration")
			Expect(testClient.Delete(ctx, controllerRegistration)).To(Succeed())
		})

		By("Wait until manager has observed ControllerRegistration")
		// Use the manager's cache to ensure it has observed the ControllerRegistration. Otherwise, the controller might
		// simply not enqueue the ControllerInstallation when extension resources get created later.
		// See https://github.com/gardener/gardener/issues/6927
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)
		}).Should(Succeed())

		By("Create ControllerInstallation")
		controllerInstallation = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "ctrlinst-",
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ControllerInstallationSpec{
				SeedRef: corev1.ObjectReference{
					Name: seedName,
				},
				RegistrationRef: corev1.ObjectReference{
					Name: controllerRegistration.Name,
				},
				DeploymentRef: &corev1.ObjectReference{
					Name: "foo-deployment",
				},
			},
		}
		Expect(testClient.Create(ctx, controllerInstallation)).To(Succeed())
		log.Info("Created ControllerInstallation for test", "controllerInstallation", client.ObjectKeyFromObject(controllerInstallation))

		DeferCleanup(func() {
			By("Delete ControllerInstallation")
			Expect(testClient.Delete(ctx, controllerInstallation)).To(Succeed())
		})

		By("Wait until manager has observed ControllerInstallation")
		// Use the manager's cache to ensure it has observed the ControllerInstallation. Otherwise, the controller might
		// simply not enqueue the ControllerInstallation when extension resources get created later.
		// See https://github.com/gardener/gardener/issues/6927
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)
		}).Should(Succeed())

		infrastructure = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "infra1-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: extensionsv1alpha1.InfrastructureSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: extensionType,
				},
			},
		}
	})

	Context("when no extension resources exist", func() {
		It("should set the Required condition to False", func() {
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
				return controllerInstallation.Status.Conditions
			}).Should(ContainCondition(OfType(gardencorev1beta1.ControllerInstallationRequired), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("NoExtensionObjects")))
		})
	})

	Context("when extension resources exist", func() {
		It("should set the Required condition to True", func() {
			By("Create Infrastructure")
			Expect(testClient.Create(ctx, infrastructure)).To(Succeed())
			log.Info("Created Infrastructure for test", "infrastructure", client.ObjectKeyFromObject(infrastructure))

			DeferCleanup(func() {
				By("Delete Infrastructure")
				Expect(testClient.Delete(ctx, infrastructure)).To(Succeed())
			})

			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
				return controllerInstallation.Status.Conditions
			}).Should(ContainCondition(OfType(gardencorev1beta1.ControllerInstallationRequired), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("ExtensionObjectsExist")))
		})

		It("should set the Required condition to False when all extension resources get deleted", func() {
			By("Create Infrastructure")
			Expect(testClient.Create(ctx, infrastructure)).To(Succeed())
			log.Info("Created Infrastructure for test", "infrastructure", client.ObjectKeyFromObject(infrastructure))

			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
				return controllerInstallation.Status.Conditions
			}).Should(ContainCondition(OfType(gardencorev1beta1.ControllerInstallationRequired), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("ExtensionObjectsExist")))

			By("Delete Infrastructure")
			Expect(testClient.Delete(ctx, infrastructure)).To(Succeed())

			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
				return controllerInstallation.Status.Conditions
			}).Should(ContainCondition(OfType(gardencorev1beta1.ControllerInstallationRequired), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("NoExtensionObjects")))
		})
	})
})
