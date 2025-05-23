// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensionscheck_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Seed ExtensionsCheck controller tests", func() {
	var (
		seed *gardencorev1beta1.Seed
		ci1  *gardencorev1beta1.ControllerInstallation
		ci2  *gardencorev1beta1.ControllerInstallation
	)

	BeforeEach(func() {
		fakeClock.SetTime(time.Now())

		By("Create Seed")
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.SeedSpec{
				Provider: gardencorev1beta1.SeedProvider{
					Region: "region",
					Type:   "providerType",
				},
				Ingress: &gardencorev1beta1.Ingress{
					Domain: "someingress.example.com",
					Controller: gardencorev1beta1.IngressController{
						Kind: "nginx",
					},
				},
				DNS: gardencorev1beta1.SeedDNS{
					Provider: &gardencorev1beta1.SeedDNSProvider{
						Type: "providerType",
						SecretRef: corev1.SecretReference{
							Name:      "some-secret",
							Namespace: "some-namespace",
						},
					},
				},
				Settings: &gardencorev1beta1.SeedSettings{
					Scheduling: &gardencorev1beta1.SeedSettingScheduling{Visible: true},
				},
				Networks: gardencorev1beta1.SeedNetworks{
					Pods:     "10.0.0.0/16",
					Services: "10.1.0.0/16",
					Nodes:    ptr.To("10.2.0.0/16"),
					ShootDefaults: &gardencorev1beta1.ShootNetworks{
						Pods:     ptr.To("100.128.0.0/11"),
						Services: ptr.To("100.72.0.0/13"),
					},
				},
			},
		}
		Expect(testClient.Create(ctx, seed)).To(Succeed())
		log.Info("Created seed for test", "seed", client.ObjectKeyFromObject(seed))

		DeferCleanup(func() {
			By("Delete Seed")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, seed))).To(Succeed())
		})

		By("Wait until manager has observed seed creation")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)
		}).Should(Succeed())

		ci1 = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "foo-1-",
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ControllerInstallationSpec{
				SeedRef: corev1.ObjectReference{
					Name: seed.Name,
				},
				RegistrationRef: corev1.ObjectReference{
					Name: "foo-registration",
				},
				DeploymentRef: &corev1.ObjectReference{
					Name: "foo-deployment",
				},
			},
		}

		ci2 = ci1.DeepCopy()
		ci2.SetGenerateName("foo-2-")
	})

	JustBeforeEach(func() {
		createAndUpdateControllerInstallation(ci1, seed, gardencorev1beta1.ConditionFalse)
		createAndUpdateControllerInstallation(ci2, seed, gardencorev1beta1.ConditionProgressing)

		By("Wait until manager has observed that ExtensionsReady condition is set to True")
		Eventually(func(g Gomega) {
			g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
			g.Expect(seed.Status.Conditions).To(ContainCondition(OfType(gardencorev1beta1.SeedExtensionsReady), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("AllExtensionsReady")))
		}).Should(Succeed())
	})

	var tests = func(failedCondition gardencorev1beta1.Condition, reason string) {
		It("should set ExtensionsReady to Progressing and eventually to False when condition threshold expires", func() {
			By("Patch conditions to False of " + ci1.Name)
			for i, condition := range ci1.Status.Conditions {
				if condition.Type == failedCondition.Type {
					ci1.Status.Conditions[i].Status = failedCondition.Status
					break
				}
			}
			Expect(testClient.Status().Update(ctx, ci1)).To(Succeed())

			By("Wait until condition is Progressing")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				g.Expect(seed.Status.Conditions).To(ContainCondition(OfType(gardencorev1beta1.SeedExtensionsReady), WithStatus(gardencorev1beta1.ConditionProgressing), WithReason(reason)))
			}).Should(Succeed())

			By("Wait until manager has observed Progressing condition")
			// Use the manager's cached client to be sure that it has observed that the ExtensionsReady condition
			// has been set to Progressing. Otherwise, it is possible that during the reconciliation which happens
			// after stepping the fake clock, an outdated Seed object with its ExtensionsReady condition set to
			// True is retrieved by the cached client. This will cause the reconciliation to set the condition to
			// Progressing again with a new timestamp. After that the condition will never change because the
			// fake clock is not stepped anymore.
			Eventually(func(g Gomega) {
				g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				g.Expect(seed.Status.Conditions).To(ContainCondition(OfType(gardencorev1beta1.SeedExtensionsReady), WithStatus(gardencorev1beta1.ConditionProgressing), WithReason(reason)))
			}).Should(Succeed())

			By("Step clock")
			fakeClock.Step(conditionThreshold * 2)

			By("Wait until condition is False")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				g.Expect(seed.Status.Conditions).To(ContainCondition(OfType(gardencorev1beta1.SeedExtensionsReady), WithStatus(gardencorev1beta1.ConditionFalse), WithReason(reason)))
			}).Should(Succeed())
		})
	}

	Context("when one ControllerInstallation becomes not valid", func() {
		tests(
			gardencorev1beta1.Condition{Type: gardencorev1beta1.ControllerInstallationValid, Status: gardencorev1beta1.ConditionFalse},
			"NotAllExtensionsValid",
		)
	})

	Context("when one ControllerInstallation is not installed", func() {
		tests(
			gardencorev1beta1.Condition{Type: gardencorev1beta1.ControllerInstallationInstalled, Status: gardencorev1beta1.ConditionFalse},
			"NotAllExtensionsInstalled",
		)
	})

	Context("when one ControllerInstallation is not healthy", func() {
		tests(
			gardencorev1beta1.Condition{Type: gardencorev1beta1.ControllerInstallationHealthy, Status: gardencorev1beta1.ConditionFalse},
			"NotAllExtensionsHealthy",
		)
	})

	Context("when one ControllerInstallation is progressing", func() {
		tests(
			gardencorev1beta1.Condition{Type: gardencorev1beta1.ControllerInstallationProgressing, Status: gardencorev1beta1.ConditionTrue},
			"SomeExtensionsProgressing",
		)
	})
})

func createAndUpdateControllerInstallation(controllerInstallation *gardencorev1beta1.ControllerInstallation, seed *gardencorev1beta1.Seed, expectedConditionAfterCreation gardencorev1beta1.ConditionStatus) {
	By("Create ControllerInstallation")
	Expect(testClient.Create(ctx, controllerInstallation)).To(Succeed(), controllerInstallation.Name+" should be created")
	log.Info("Created ControllerInstallation for test", "controllerInstallation", client.ObjectKeyFromObject(controllerInstallation))

	DeferCleanup(func() {
		By("Delete ControllerInstallation")
		Expect(client.IgnoreNotFound(testClient.Delete(ctx, controllerInstallation))).To(Succeed())
	})

	By("Wait until ExtensionsReady condition is set to " + string(expectedConditionAfterCreation))
	Eventually(func(g Gomega) {
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
		g.Expect(seed.Status.Conditions).To(ContainCondition(OfType(gardencorev1beta1.SeedExtensionsReady), WithStatus(expectedConditionAfterCreation), WithReason("NotAllExtensionsInstalled")))
	}).Should(Succeed(), "before update of "+controllerInstallation.Name)

	By("Update ControllerInstallation with successful status")
	controllerInstallation.Status = gardencorev1beta1.ControllerInstallationStatus{
		Conditions: []gardencorev1beta1.Condition{
			{Type: "Valid", Status: gardencorev1beta1.ConditionTrue},
			{Type: "Installed", Status: gardencorev1beta1.ConditionTrue},
			{Type: "Healthy", Status: gardencorev1beta1.ConditionTrue},
			{Type: "Progressing", Status: gardencorev1beta1.ConditionFalse},
		},
	}
	Expect(testClient.Status().Update(ctx, controllerInstallation)).To(Succeed(), controllerInstallation.Name+" should be updated")
	log.Info("Updated ControllerInstallation for test", "controllerInstallation", client.ObjectKeyFromObject(controllerInstallation))

	By("Wait until ExtensionsReady condition is set to True")
	Eventually(func(g Gomega) {
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
		g.Expect(seed.Status.Conditions).To(ContainCondition(OfType(gardencorev1beta1.SeedExtensionsReady), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("AllExtensionsReady")))
	}).Should(Succeed(), "after creation and update of "+controllerInstallation.Name)
}
