// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ControllerInstallation Care controller tests", func() {
	var controllerInstallation *gardencorev1beta1.ControllerInstallation

	BeforeEach(func() {
		By("Create ControllerInstallation")
		controllerInstallation = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "foo-",
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ControllerInstallationSpec{
				SeedRef: corev1.ObjectReference{
					Name: "foo-seed",
				},
				RegistrationRef: corev1.ObjectReference{
					Name: "foo-registration",
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
	})

	Context("when ManagedResources for the ControllerInstallation do not exist", func() {
		It("should set conditions to Unknown", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
				g.Expect(controllerInstallation.Status.Conditions).To(ConsistOf(
					And(OfType(gardencorev1beta1.ControllerInstallationInstalled), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("SeedReadError"), withMessageSubstrings("Failed to get ManagedResource", "not found")),
					And(OfType(gardencorev1beta1.ControllerInstallationHealthy), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("SeedReadError"), withMessageSubstrings("Failed to get ManagedResource", "not found")),
					And(OfType(gardencorev1beta1.ControllerInstallationProgressing), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("SeedReadError"), withMessageSubstrings("Failed to get ManagedResource", "not found")),
				))
			}).Should(Succeed())
		})
	})

	Context("when ManagedResources for the ControllerInstallation exist", func() {
		var managedResource *resourcesv1alpha1.ManagedResource

		BeforeEach(func() {
			By("Create ManagedResource")
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controllerInstallation.Name,
					Namespace: gardenNamespace.Name,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs: []corev1.LocalObjectReference{{
						Name: "foo-secret",
					}},
				},
			}
			Expect(testClient.Create(ctx, managedResource)).To(Succeed())
			log.Info("Created ManagedResource for test", "managedResource", client.ObjectKeyFromObject(managedResource))

			DeferCleanup(func() {
				By("Delete ManagedResource")
				Expect(testClient.Delete(ctx, managedResource)).To(Succeed())
			})
		})

		Context("when generation of ManagedResource is outdated", func() {
			It("shout set Installed condition to False with generation outdated error", func() {
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
					g.Expect(controllerInstallation.Status.Conditions).To(ContainCondition(OfType(gardencorev1beta1.ControllerInstallationInstalled), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("InstallationPending"), withMessageSubstrings("observed generation of managed resource", "outdated (0/1)")))
				}).Should(Succeed())
			})
		})

		Context("when generation of ManagedResource is up to date", func() {
			BeforeEach(func() {
				managedResource.Status.ObservedGeneration = managedResource.Generation
				Expect(testClient.Status().Update(ctx, managedResource)).To(Succeed())
			})

			It("should set conditions to failed when ManagedResource conditions do not exist yet", func() {
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
					g.Expect(controllerInstallation.Status.Conditions).To(ConsistOf(
						And(OfType(gardencorev1beta1.ControllerInstallationInstalled), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("InstallationPending"), withMessageSubstrings("condition", "has not been reported")),
						And(OfType(gardencorev1beta1.ControllerInstallationHealthy), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("ControllerNotHealthy"), withMessageSubstrings("condition", "has not been reported")),
						And(OfType(gardencorev1beta1.ControllerInstallationProgressing), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("ControllerNotRolledOut"), withMessageSubstrings("condition", "has not been reported")),
					))
				}).Should(Succeed())
			})

			It("should set conditions to failed when conditions of ManagedResource are not successful yet", func() {
				managedResource.Status.Conditions = []gardencorev1beta1.Condition{
					{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionFalse, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
					{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionFalse, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
					{Type: resourcesv1alpha1.ResourcesProgressing, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
				}
				Expect(testClient.Status().Update(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
					g.Expect(controllerInstallation.Status.Conditions).To(ConsistOf(
						And(OfType(gardencorev1beta1.ControllerInstallationInstalled), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("InstallationPending")),
						And(OfType(gardencorev1beta1.ControllerInstallationHealthy), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("ControllerNotHealthy")),
						And(OfType(gardencorev1beta1.ControllerInstallationProgressing), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("ControllerNotRolledOut")),
					))
				}).Should(Succeed())
			})

			It("should set conditions to successful when conditions of ManagedResource become successful", func() {
				managedResource.Status.Conditions = []gardencorev1beta1.Condition{
					{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
					{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
					{Type: resourcesv1alpha1.ResourcesProgressing, Status: gardencorev1beta1.ConditionFalse, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
				}
				Expect(testClient.Status().Update(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
					g.Expect(controllerInstallation.Status.Conditions).To(ConsistOf(
						And(OfType(gardencorev1beta1.ControllerInstallationInstalled), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("InstallationSuccessful")),
						And(OfType(gardencorev1beta1.ControllerInstallationHealthy), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("ControllerHealthy")),
						And(OfType(gardencorev1beta1.ControllerInstallationProgressing), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("ControllerRolledOut")),
					))
				}).Should(Succeed())
			})
		})
	})
})

func withMessageSubstrings(messages ...string) gomegatypes.GomegaMatcher {
	var substringMatchers = make([]gomegatypes.GomegaMatcher, 0, len(messages))
	for _, message := range messages {
		substringMatchers = append(substringMatchers, ContainSubstring(message))
	}
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Message": SatisfyAll(substringMatchers...),
	})
}
