// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootstate_test

import (
	"context"
	"time"

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
			By("Delete Shoot")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot))).To(Succeed())

			By("Delete ShootState")
			Expect(controllerutils.RemoveFinalizers(ctx, testClient, shootState, shootstate.FinalizerName)).To(Succeed())
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shootState))).To(Succeed())

			By("Ensure ShootState is gone")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)
			}).Should(BeNotFoundError())

			By("Ensure Shoot is gone")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
			}).Should(BeNotFoundError())

		})
	})

	When("migrating Shoot to another Seed", func() {
		Context("should reconcile ShootState object", func() {
			BeforeEach(func() {
				By("Mark Shoot for migration")
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.SeedName = &targetSeedName
				Expect(testClient.SubResource("binding").Patch(ctx, shoot, patch)).To(Succeed())
			})

			It("should add finalizer if not present", func() {
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:     gardencorev1beta1.LastOperationTypeMigrate,
					State:    gardencorev1beta1.LastOperationStateProcessing,
					Progress: 0,
				}
				Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

				By("Should attach finalizer")
				ctxTimeOut, ctxCancel := context.WithTimeout(ctx, 60*5*time.Second)
				defer ctxCancel()

				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
					return shootState.Finalizers
				}).WithContext(ctxTimeOut).Should(ConsistOf(shootstate.FinalizerName))
			})

			It("should remove finalizer when Shoot restores successfully", func() {
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:     gardencorev1beta1.LastOperationTypeRestore,
					State:    gardencorev1beta1.LastOperationStateSucceeded,
					Progress: 100,
				}
				Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

				shootState.Finalizers = append(shoot.Finalizers, shootstate.FinalizerName)
				Expect(testClient.Update(ctx, shootState)).To(Succeed())

				By("Should remove finalizer")
				ctxTimeOut, ctxCancel := context.WithTimeout(ctx, 60*5*time.Second)
				defer ctxCancel()

				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
					return shootState.Finalizers
				}).WithContext(ctxTimeOut).ShouldNot(ConsistOf(shootstate.FinalizerName))
			})
		})
	})
})
