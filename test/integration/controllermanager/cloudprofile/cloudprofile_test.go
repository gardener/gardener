// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprofile_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CloudProfile controller tests", func() {
	var (
		cloudProfile *gardencorev1beta1.CloudProfile
		shoot        *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		cloudProfile = &gardencorev1beta1.CloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.CloudProfileSpec{
				Type: "some-provider",
				Kubernetes: gardencorev1beta1.KubernetesSettings{
					Versions: []gardencorev1beta1.ExpirableVersion{{Version: "1.2.3"}},
				},
				MachineImages: []gardencorev1beta1.MachineImage{
					{
						Name: "some-image",
						Versions: []gardencorev1beta1.MachineImageVersion{
							{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "4.5.6"}},
						},
					},
				},
				MachineTypes: []gardencorev1beta1.MachineType{{
					Name:   "some-type",
					CPU:    resource.MustParse("1"),
					GPU:    resource.MustParse("0"),
					Memory: resource.MustParse("1Gi"),
				}},
				Regions: []gardencorev1beta1.Region{
					{Name: "some-region"},
				},
			},
		}

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
				Namespace:    testNamespace.Name,
			},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: ptr.To("my-provider-account"),
				Region:            "foo-region",
				Provider: gardencorev1beta1.Provider{
					Type: cloudProfile.Spec.Type,
					Workers: []gardencorev1beta1.Worker{
						{
							Name:    "cpu-worker",
							Minimum: 2,
							Maximum: 2,
							Machine: gardencorev1beta1.Machine{Type: "large"},
						},
					},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{Version: "1.26.1"},
				Networking: &gardencorev1beta1.Networking{Type: ptr.To("foo-networking")},
			},
		}
	})

	JustBeforeEach(func() {
		By("Create CloudProfile")
		Expect(testClient.Create(ctx, cloudProfile)).To(Succeed())
		log.Info("Created CloudProfile for test", "cloudProfile", client.ObjectKeyFromObject(cloudProfile))

		DeferCleanup(func() {
			By("Delete CloudProfile")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, cloudProfile))).To(Succeed())
		})

		if shoot != nil {
			By("Create Shoot")
			shoot.Spec.CloudProfileName = &cloudProfile.Name
			Expect(testClient.Create(ctx, shoot)).To(Succeed())
			log.Info("Created shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

			By("Wait until manager has observed Shoot")
			// Use the manager's cache to ensure it has observed the Shoot.
			// Otherwise, the controller might clean up the CloudProfile too early because it thinks all referencing
			// Shoots are gone. Similar to https://github.com/gardener/gardener/issues/6486 and
			// https://github.com/gardener/gardener/issues/6607.
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), &gardencorev1beta1.Shoot{})
			}).Should(Succeed())

			DeferCleanup(func() {
				By("Delete Shoot")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot))).To(Succeed())
			})
		}
	})

	Context("no shoot referencing the CloudProfile", func() {
		BeforeEach(func() {
			shoot = nil
		})

		It("should add the finalizer and release it on deletion", func() {
			By("Ensure finalizer got added")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(cloudProfile), cloudProfile)).To(Succeed())
				g.Expect(cloudProfile.Finalizers).To(ConsistOf("gardener"))
			}).Should(Succeed())

			By("Delete CloudProfile")
			Expect(testClient.Delete(ctx, cloudProfile)).To(Succeed())

			By("Ensure CloudProfile is released")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(cloudProfile), cloudProfile)
			}).Should(BeNotFoundError())
		})
	})

	Context("shoots referencing the CloudProfile", func() {
		JustBeforeEach(func() {
			By("Ensure finalizer got added")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(cloudProfile), cloudProfile)).To(Succeed())
				g.Expect(cloudProfile.Finalizers).To(ConsistOf("gardener"))
			}).Should(Succeed())

			By("Delete CloudProfile")
			Expect(testClient.Delete(ctx, cloudProfile)).To(Succeed())
		})

		It("should add the finalizer and not release it on deletion since there still is a referencing shoot", func() {
			By("Ensure CloudProfile is not released")
			Consistently(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(cloudProfile), cloudProfile)
			}).Should(Succeed())
		})

		It("should add the finalizer and release it on deletion after the shoot got deleted", func() {
			By("Delete Shoot")
			Expect(testClient.Delete(ctx, shoot)).To(Succeed())

			By("Ensure CloudProfile is released")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(cloudProfile), cloudProfile)
			}).Should(BeNotFoundError())
		})
	})
})
