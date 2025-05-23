// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package virtual_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Extension Required Virtual controller tests", func() {
	var (
		extension *operatorv1alpha1.Extension

		providerControllerInstallation, providerControllerInstallation2 *gardencorev1beta1.ControllerInstallation
		dnsControllerInstallation                                       *gardencorev1beta1.ControllerInstallation
	)

	BeforeEach(func() {
		extension = &operatorv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name: testRunID,
				Labels: map[string]string{
					testID: testRunID,
				},
			},
		}

		providerControllerInstallation = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				Name: testRunID + "-1",
			},
		}

		providerControllerInstallation2 = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				Name: testRunID + "-2",
			},
		}

		dnsControllerInstallation = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				Name: testRunID + "-3",
			},
		}

		DeferCleanup(func() {
			for _, controllerInstallation := range []*gardencorev1beta1.ControllerInstallation{
				providerControllerInstallation,
				providerControllerInstallation2,
				dnsControllerInstallation,
			} {
				Expect(testClient.Delete(ctx, controllerInstallation)).To(Or(Succeed(), BeNotFoundError()), fmt.Sprintf("ControllerInstallation %s should get deleted", controllerInstallation.Name))
			}
		})
	})

	It("should reconcile the extensions and calculate the expected required status", func() {
		By("Create extensions")
		providerExtension := extension.DeepCopy()
		providerExtension.Name += "-provider"
		Expect(testClient.Create(ctx, providerExtension)).To(Succeed())
		log.Info("Created extension", "garden", providerExtension.GetName())

		dnsExtension := extension.DeepCopy()
		dnsExtension.Name += "-dns"
		Expect(testClient.Create(ctx, dnsExtension)).To(Succeed())
		log.Info("Created extension", "garden", dnsExtension.GetName())

		By("Ensure just created extensions are reported as not required")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(providerExtension), providerExtension)).To(Succeed())
			g.Expect(providerExtension.Status.Conditions).Should(ContainCondition(
				OfType("RequiredVirtual"),
				WithStatus("False"),
			))

			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dnsExtension), dnsExtension)).To(Succeed())
			g.Expect(providerExtension.Status.Conditions).Should(ContainCondition(
				OfType("RequiredVirtual"),
				WithStatus("False"),
			))
		}).Should(Succeed())

		By("Ensure provider extension is reported as required when at least one required ControllerInstallation exist")
		Expect(createControllerInstallation(ctx, testClient, providerControllerInstallation, providerExtension.Name)).To(Succeed())
		Expect(updateRequiredCondition(ctx, testClient, providerControllerInstallation, "True")).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(providerExtension), providerExtension)).To(Succeed())
			g.Expect(providerExtension.Status.Conditions).Should(ContainCondition(
				OfType("RequiredVirtual"),
				WithStatus("True"),
			))

			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dnsExtension), dnsExtension)).To(Succeed())
			g.Expect(dnsExtension.Status.Conditions).Should(ContainCondition(
				OfType("RequiredVirtual"),
				WithStatus("False"),
			))
		}).Should(Succeed())

		By("Ensure provider extension is still reported as required when at least one required ControllerInstallation exist")
		Expect(createControllerInstallation(ctx, testClient, providerControllerInstallation2, providerExtension.Name)).To(Succeed())
		Expect(updateRequiredCondition(ctx, testClient, providerControllerInstallation2, "False")).To(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(providerExtension), providerExtension)).To(Succeed())
			g.Expect(providerExtension.Status.Conditions).Should(ContainCondition(
				OfType("RequiredVirtual"),
				WithStatus("True"),
			))
		}).Should(Succeed())

		By("Ensure provider extension is still reported as required when one ControllerInstallation is deleted")
		Expect(updateRequiredCondition(ctx, testClient, providerControllerInstallation2, "True")).To(Succeed())

		Eventually(func(g Gomega) {
			cachedControllerInstallation := &gardencorev1beta1.ControllerInstallation{}
			g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(providerControllerInstallation2), cachedControllerInstallation)).To(Succeed())
			g.Expect(providerControllerInstallation2.Status.Conditions).Should(ContainCondition(
				OfType("Required"),
				WithStatus("True"),
			))
		}).Should(Succeed())

		Expect(testClient.Delete(ctx, providerControllerInstallation)).To(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(providerControllerInstallation), &gardencorev1beta1.ControllerInstallation{})).To(BeNotFoundError())
		}).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(providerExtension), providerExtension)).To(Succeed())
			g.Expect(providerExtension.Status.Conditions).Should(ContainCondition(
				OfType("RequiredVirtual"),
				WithStatus("True"),
			))
		}).Should(Succeed())

		By("Ensure provider extension is reported as not required when ControllerInstallation is updated as not required")
		Expect(updateRequiredCondition(ctx, testClient, providerControllerInstallation2, "False")).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(providerExtension), providerExtension)).To(Succeed())
			g.Expect(providerExtension.Status.Conditions).Should(ContainCondition(
				OfType("RequiredVirtual"),
				WithStatus("False"),
			))
		}).Should(Succeed())

		By("Ensure provider extension is reported as not required when at least one not required ControllerInstallation exist")
		Expect(createControllerInstallation(ctx, testClient, dnsControllerInstallation, dnsExtension.Name)).To(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dnsExtension), dnsExtension)).To(Succeed())
			g.Expect(dnsExtension.Status.Conditions).Should(ContainCondition(
				OfType("RequiredVirtual"),
				WithStatus("False"),
			))
		}).Should(Succeed())

		Expect(updateRequiredCondition(ctx, testClient, dnsControllerInstallation, "False")).To(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dnsExtension), dnsExtension)).To(Succeed())
			g.Expect(dnsExtension.Status.Conditions).Should(ContainCondition(
				OfType("RequiredVirtual"),
				WithStatus("False"),
			))
		}).Should(Succeed())
	})
})

func createControllerInstallation(ctx context.Context, cl client.Client, controllerInstallation *gardencorev1beta1.ControllerInstallation, extensionName string) error {
	metav1.SetMetaDataLabel(&controllerInstallation.ObjectMeta, testID, testRunID)
	controllerInstallation.Spec = gardencorev1beta1.ControllerInstallationSpec{
		RegistrationRef: corev1.ObjectReference{
			Name: extensionName,
		},
		SeedRef: corev1.ObjectReference{
			Name: "local",
		},
	}

	return cl.Create(ctx, controllerInstallation)
}

func updateRequiredCondition(ctx context.Context, cl client.Client, controllerInstallation *gardencorev1beta1.ControllerInstallation, status gardencorev1beta1.ConditionStatus) error {
	controllerInstallation.Status = gardencorev1beta1.ControllerInstallationStatus{
		Conditions: v1beta1helper.MergeConditions(controllerInstallation.Status.Conditions, gardencorev1beta1.Condition{
			Type:   "Required",
			Status: status,
		}),
	}

	return cl.Status().Update(ctx, controllerInstallation)
}
