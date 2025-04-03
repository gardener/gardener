// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootstate_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shootstate"
	"github.com/gardener/gardener/pkg/controllerutils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ShootState controller test", func() {
	var (
		shoot          *gardencorev1beta1.Shoot
		shootState     *gardencorev1beta1.ShootState
		targetSeedName = "target-seed"
	)

	BeforeEach(func() {
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: ptr.To("secretbinding"),
				CloudProfileName:  ptr.To("cloudprofile1"),
				Region:            "europe-central-1",
				Provider: gardencorev1beta1.Provider{
					Type: "foo-provider",
					Workers: []gardencorev1beta1.Worker{
						{
							Name:    "cpu-worker",
							Minimum: 3,
							Maximum: 3,
							Machine: gardencorev1beta1.Machine{
								Type: "large",
							},
						},
					},
				},
				DNS: &gardencorev1beta1.DNS{
					Domain: ptr.To("some-domain.example.com"),
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.32.0",
				},
				Networking: &gardencorev1beta1.Networking{
					Type: ptr.To("foo-networking"),
				},
				SeedName: ptr.To("source-seed"),
			},
		}

		By("Create Shoot")
		Expect(testClient.Create(ctx, shoot)).To(Succeed())

		patch := client.MergeFrom(shoot.DeepCopy())
		shoot.Status.SeedName = shoot.Spec.SeedName
		Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

		By("Create ShootState")
		shootState = &gardencorev1beta1.ShootState{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace.Name,
				Name:      shoot.Name,
				Labels:    map[string]string{testID: testRunID},
			},
		}
		Expect(testClient.Create(ctx, shootState)).To(Succeed())

		DeferCleanup(func() {
			By("Delete ShootState")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shootState))).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Or(Succeed(), BeNotFoundError()))
				g.Expect(controllerutils.RemoveFinalizers(ctx, testClient, shootState, shootstate.FinalizerName)).To(Succeed())
			}).Should(Succeed())

			By("Ensure ShootState is gone")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)
			}).Should(BeNotFoundError())

			By("Delete Shoot")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot))).To(Succeed())

			By("Ensure Shoot is gone")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
			}).Should(BeNotFoundError())

		})
	})

	When("migrating Shoot to another Seed", func() {
		BeforeEach(func() {
			By("Mark Shoot for migration")
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Spec.SeedName = &targetSeedName
			Expect(testClient.SubResource("binding").Patch(ctx, shoot, patch)).To(Succeed())
		})

		When("Shoot last operation is Migrate", func() {
			BeforeEach(func() {
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeMigrate,
					State: gardencorev1beta1.LastOperationStateProcessing,
				}
				By("Update the Shoot's last operation")
				Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())
			})

			It("should add finalizer if not present", func() {
				By("Verify the finalizer is present")
				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
					return shootState.Finalizers
				}).Should(ConsistOf(shootstate.FinalizerName))
			})

			It("should not add/duplicate finalizer if already present", func() {
				addFinalizer(shootState)

				By("Verify the finalizer is present")
				Consistently(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
					return shootState.Finalizers
				}).Should(ConsistOf(shootstate.FinalizerName))
			})
		})

		When("Shoot last operation is Restore", func() {
			BeforeEach(func() {
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeRestore,
					State: gardencorev1beta1.LastOperationStateProcessing,
				}
				By("Update the Shoot's last operation")
				Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

			})

			It("should add finalizer if not present and operation has not succeeded", func() {
				By("Verify the finalizer is present")
				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
					return shootState.Finalizers
				}).Should(ConsistOf(shootstate.FinalizerName))
			})

			It("should not duplicate finalizer if already present", func() {
				addFinalizer(shootState)

				By("Verify the finalizer is present")
				Consistently(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
					return shootState.Finalizers
				}).Should(ConsistOf(shootstate.FinalizerName))
			})

			It("should remove finalizer if present and operation has succeeded", func() {
				addFinalizer(shootState)

				By("Update the Shoot's last operation State")
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
				Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

				By("Verify the finalizer is removed")
				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
					return shootState.Finalizers
				}).ShouldNot(ConsistOf(shootstate.FinalizerName))
			})

			It("should not remove finalizer if present and operation has not succeeded ", func() {
				addFinalizer(shootState)

				By("Verify the finalizer is present")
				Consistently(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
					return shootState.Finalizers
				}).Should(ConsistOf(shootstate.FinalizerName))
			})
		})

		When("Shoot last operation is Reconcile", func() {
			BeforeEach(func() {
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeReconcile,
					State: gardencorev1beta1.LastOperationStateProcessing,
				}
				By("Update the Shoot's last operation")
				Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())
			})

			It("should remove finalizer if present", func() {
				addFinalizer(shootState)

				By("Verify the finalizer is removed")
				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
					return shootState.Finalizers
				}).ShouldNot(ConsistOf(shootstate.FinalizerName))
			})
		})
	})
})

func addFinalizer(shootState *gardencorev1beta1.ShootState) {
	By("Add ShootState finalizer")
	EventuallyWithOffset(1, func(g Gomega) {
		g.ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
		g.ExpectWithOffset(1, controllerutils.AddFinalizers(ctx, testClient, shootState, shootstate.FinalizerName)).To(Succeed())
	}).Should(Succeed())
}
