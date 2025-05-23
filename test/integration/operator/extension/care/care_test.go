// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Extension Care controller tests", func() {
	var extension *operatorv1alpha1.Extension

	When("Extension exists", func() {
		extensionName := "foo"
		managedResourceName := "extension-foo-garden"

		BeforeEach(func() {
			By("Create Extension")
			extension = &operatorv1alpha1.Extension{
				ObjectMeta: metav1.ObjectMeta{
					Name: extensionName,
				},
			}
			Expect(testClient.Create(ctx, extension)).To(Succeed())
			log.Info("Created Extension for test", "extension", client.ObjectKeyFromObject(extension))

			By("Set Extension to status required")
			now := metav1.Now()
			extension.Status = operatorv1alpha1.ExtensionStatus{
				Conditions: []gardencorev1beta1.Condition{
					{Type: operatorv1alpha1.ExtensionRequiredRuntime, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: now, LastUpdateTime: now},
				},
			}
			Expect(testClient.Status().Update(ctx, extension)).To(Succeed())

			DeferCleanup(func() {
				By("Delete Extension")
				Expect(testClient.Delete(ctx, &operatorv1alpha1.Extension{ObjectMeta: metav1.ObjectMeta{Name: extensionName}})).To(Succeed())
			})

			By("Create ManagedResource for runtime cluster")
			managedResource := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceName,
					Namespace: testNamespace.Name,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class:      ptr.To("seed"),
					SecretRefs: []corev1.LocalObjectReference{{Name: "foo-secret"}},
				},
			}
			Expect(testClient.Create(ctx, managedResource)).To(Succeed())
			log.Info("Created ManagedResource", "managedResource", client.ObjectKeyFromObject(managedResource))

			DeferCleanup(func() {
				By("Delete ManagedResource for extension in runtime cluster")
				Expect(testClient.Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: testNamespace.Name}})).To(Succeed())
			})
		})

		It("should set condition to False because all ManagedResource statuses are outdated", func() {
			By("Expect ExtensionHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
				return extension.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.ExtensionHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("OutdatedStatus"),
				WithMessageSubstrings("observed generation of managed resource"),
			))
		})

		It("should set condition to True because all ManagedResource statuses are healthy", func() {
			updateManagedResourceStatusToHealthy(managedResourceName)

			By("Expect ExtensionHealthy condition to be True")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
				return extension.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.ExtensionHealthy),
				WithStatus(gardencorev1beta1.ConditionTrue),
				WithReason("ExtensionComponentsRunning"),
				WithMessageSubstrings("All extension components are healthy."),
			))
		})
	})

	Context("when Extension admission exists", func() {
		extensionName := "bar"
		managedResourceRuntimeName := "extension-admission-runtime-bar"
		managedResourceVirtualName := "extension-admission-virtual-bar"

		BeforeEach(func() {
			By("Create Extension")
			extension = &operatorv1alpha1.Extension{
				ObjectMeta: metav1.ObjectMeta{
					Name: extensionName,
				},
				Spec: operatorv1alpha1.ExtensionSpec{
					Deployment: &operatorv1alpha1.Deployment{
						AdmissionDeployment: &operatorv1alpha1.AdmissionDeploymentSpec{},
					},
				},
			}
			Expect(testClient.Create(ctx, extension)).To(Succeed())
			log.Info("Created Extension for test", "extension", client.ObjectKeyFromObject(extension))

			DeferCleanup(func() {
				By("Delete Extension")
				Expect(testClient.Delete(ctx, &operatorv1alpha1.Extension{ObjectMeta: metav1.ObjectMeta{Name: extensionName}})).To(Succeed())
			})

			By("Create ManagedResource for runtime cluster")
			managedResourceRuntime := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceRuntimeName,
					Namespace: testNamespace.Name,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class:      ptr.To("seed"),
					SecretRefs: []corev1.LocalObjectReference{{Name: "bar-runtime-secret"}},
				},
			}
			Expect(testClient.Create(ctx, managedResourceRuntime)).To(Succeed())
			log.Info("Created ManagedResource", "managedResource", client.ObjectKeyFromObject(managedResourceRuntime))

			DeferCleanup(func() {
				By("Delete ManagedResource for extension admission in runtime cluster")
				Expect(testClient.Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceRuntimeName, Namespace: testNamespace.Name}})).To(Succeed())
			})

			By("Create ManagedResource for virtual cluster")
			managedResourceVirtual := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceVirtualName,
					Namespace: testNamespace.Name,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs: []corev1.LocalObjectReference{{Name: "bar-virtual-secret"}},
				},
			}
			Expect(testClient.Create(ctx, managedResourceVirtual)).To(Succeed())
			log.Info("Created ManagedResource", "managedResource", client.ObjectKeyFromObject(managedResourceVirtual))

			DeferCleanup(func() {
				By("Delete ManagedResource for extension admission in virtual cluster")
				Expect(testClient.Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceVirtualName, Namespace: testNamespace.Name}})).To(Succeed())
			})
		})

		It("should set condition to False because all ManagedResource statuses are outdated", func() {
			By("Expect ExtensionHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
				return extension.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.ExtensionAdmissionHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("OutdatedStatus"),
				WithMessageSubstrings("observed generation of managed resource"),
			))
		})

		It("should set condition to False because status of virtual ManagedResource statuses is outdated", func() {
			updateManagedResourceStatusToHealthy(managedResourceRuntimeName)

			By("Expect ExtensionHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
				return extension.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.ExtensionAdmissionHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("OutdatedStatus"),
				WithMessageSubstrings("observed generation of managed resource"),
			))
		})

		It("should set condition to False because status of runtime ManagedResource statuses is outdated", func() {
			updateManagedResourceStatusToHealthy(managedResourceVirtualName)

			By("Expect ExtensionHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
				return extension.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.ExtensionAdmissionHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("OutdatedStatus"),
				WithMessageSubstrings("observed generation of managed resource"),
			))
		})

		It("should set condition to True because all ManagedResource statuses are healthy", func() {
			updateManagedResourceStatusToHealthy(managedResourceVirtualName)
			updateManagedResourceStatusToHealthy(managedResourceRuntimeName)

			By("Expect ExtensionHealthy condition to be True")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
				return extension.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.ExtensionAdmissionHealthy),
				WithStatus(gardencorev1beta1.ConditionTrue),
				WithReason("ExtensionAdmissionComponentsRunning"),
				WithMessageSubstrings("All extension admission components are healthy."),
			))
		})
	})

	Context("when Controller Registration and Installation exist", func() {
		extensionName := "foobar"
		controllerinstallationName := "foobar"

		BeforeEach(func() {
			By("Create Extension")
			extension = &operatorv1alpha1.Extension{
				ObjectMeta: metav1.ObjectMeta{
					Name: extensionName,
				},
			}
			Expect(testClient.Create(ctx, extension)).To(Succeed())
			log.Info("Created Extension for test", "extension", client.ObjectKeyFromObject(extension))

			By("Set Extension to status required")
			now := metav1.Now()
			extension.Status = operatorv1alpha1.ExtensionStatus{
				Conditions: []gardencorev1beta1.Condition{
					{Type: operatorv1alpha1.ExtensionRequiredVirtual, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: now, LastUpdateTime: now},
				},
			}
			Expect(testClient.Status().Update(ctx, extension)).To(Succeed())

			DeferCleanup(func() {
				By("Delete Extension")
				Expect(testClient.Delete(ctx, &operatorv1alpha1.Extension{ObjectMeta: metav1.ObjectMeta{Name: extensionName}})).To(Succeed())
			})

			By("Create ControllerRegistration")
			controllerRegistration := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foobar",
				},
			}
			Expect(testClient.Create(ctx, controllerRegistration)).To(Succeed())
			log.Info("Created ControllerRegistration", "controllerRegistration", client.ObjectKeyFromObject(controllerRegistration))

			DeferCleanup(func() {
				By("Delete ControllerRegistration")
				Expect(testClient.Delete(ctx, &gardencorev1beta1.ControllerRegistration{ObjectMeta: metav1.ObjectMeta{Name: controllerRegistration.Name}})).To(Succeed())
			})

			By("Create ControllerInstallation")
			controllerInstallation := &gardencorev1beta1.ControllerInstallation{
				ObjectMeta: metav1.ObjectMeta{
					Name: controllerinstallationName,
				},
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					RegistrationRef: corev1.ObjectReference{
						Name:            controllerRegistration.Name,
						ResourceVersion: controllerRegistration.ResourceVersion,
					},
					SeedRef: corev1.ObjectReference{
						Name:            "foo",
						ResourceVersion: "0",
					},
				},
			}
			Expect(testClient.Create(ctx, controllerInstallation)).To(Succeed())
			log.Info("Created ControllerInstallation", "controllerInstallation", client.ObjectKeyFromObject(controllerInstallation))

			DeferCleanup(func() {
				By("Delete ControllerInstallation")
				Expect(testClient.Delete(ctx, &gardencorev1beta1.ControllerInstallation{ObjectMeta: metav1.ObjectMeta{Name: controllerinstallationName}})).To(Succeed())
			})
		})

		It("should set condition to False because all ControllerRegistrations are outdated", func() {
			updateControllerInstallationToOutdated(controllerinstallationName)

			By("Expect ExtensionHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
				return extension.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.ControllerInstallationsHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("OutdatedControllerRegistration"),
				WithMessageSubstrings("observed resource version of controller registration"),
			))
		})

		It("should set condition to True because all ControllerInstallations statuses are healthy", func() {
			updateControllerInstallationStatusToHealthy(controllerinstallationName)

			By("Expect ExtensionHealthy condition to be True")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
				return extension.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.ControllerInstallationsHealthy),
				WithStatus(gardencorev1beta1.ConditionTrue),
				WithReason("ControllerInstallationsRunning"),
				WithMessageSubstrings("All controller installations are healthy."),
			))
		})
	})
})

func updateManagedResourceStatusToHealthy(name string) {
	By("Update status to healthy for ManagedResource " + name)
	managedResource := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace.Name}}
	ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())

	managedResource.Status.ObservedGeneration = managedResource.Generation
	managedResource.Status.Conditions = []gardencorev1beta1.Condition{
		{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
		{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
		{Type: resourcesv1alpha1.ResourcesProgressing, Status: gardencorev1beta1.ConditionFalse, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
	}
	ExpectWithOffset(1, testClient.Status().Update(ctx, managedResource)).To(Succeed())
}

func updateControllerInstallationStatusToHealthy(name string) {
	By("Update status to healthy for ControllerInstallation " + name)
	controllerInstallation := &gardencorev1beta1.ControllerInstallation{ObjectMeta: metav1.ObjectMeta{Name: name}}
	ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())

	controllerInstallation.Status.Conditions = []gardencorev1beta1.Condition{
		{Type: gardencorev1beta1.ControllerInstallationValid, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
		{Type: gardencorev1beta1.ControllerInstallationInstalled, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
		{Type: gardencorev1beta1.ControllerInstallationHealthy, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
		{Type: gardencorev1beta1.ControllerInstallationProgressing, Status: gardencorev1beta1.ConditionFalse, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
	}
	ExpectWithOffset(1, testClient.Status().Update(ctx, controllerInstallation)).To(Succeed())
}

func updateControllerInstallationToOutdated(name string) {
	By("Update ControllerInstallation " + name + " to outdated")
	controllerInstallation := &gardencorev1beta1.ControllerInstallation{ObjectMeta: metav1.ObjectMeta{Name: name}}
	ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())

	controllerInstallation.Spec.RegistrationRef.ResourceVersion = "0"

	ExpectWithOffset(1, testClient.Update(ctx, controllerInstallation)).To(Succeed())
}
