// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/state"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Shoot State controller tests", func() {
	var (
		shoot      *gardencorev1beta1.Shoot
		shootState *gardencorev1beta1.ShootState
		secret     *corev1.Secret

		lastOperation *gardencorev1beta1.LastOperation
	)

	BeforeEach(func() {
		DeferCleanup(test.WithVars(
			&state.RequeueWhenShootIsNotReadyForBackup, 100*time.Millisecond,
			&state.JitterDuration, time.Duration(0),
		))

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "shoot-",
				Namespace:    projectNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ShootSpec{
				Region:           "local",
				CloudProfileName: ptr.To("local"),
				Kubernetes:       gardencorev1beta1.Kubernetes{Version: "1.27.1"},
				Provider:         gardencorev1beta1.Provider{Type: "local"},
				SeedName:         &seedName,
			},
		}
		lastOperation = &gardencorev1beta1.LastOperation{
			Type:  gardencorev1beta1.LastOperationTypeReconcile,
			State: gardencorev1beta1.LastOperationStateSucceeded,
		}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-secret",
				Namespace: seedNamespace.Name,
				Labels: map[string]string{
					testID:    testRunID,
					"persist": "true",
				},
			},
		}
	})

	createPersistableSecret := func() {
		By("Create secret for next backup")
		ExpectWithOffset(1, testClient.Create(ctx, secret)).To(Succeed())
		log.Info("Created Secret for test", "secret", secret.Name)

		DeferCleanup(func() {
			By("Delete Secret")
			ExpectWithOffset(1, testClient.Delete(ctx, secret)).To(Succeed())

			By("Ensure Secret is gone")
			EventuallyWithOffset(1, func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)
			}).Should(BeNotFoundError())
		})
	}

	JustBeforeEach(func() {
		By("Create Shoot")
		Expect(testClient.Create(ctx, shoot)).To(Succeed())
		log.Info("Created Shoot for test", "shoot", shoot.Name)
		shootState = &gardencorev1beta1.ShootState{ObjectMeta: shoot.ObjectMeta}

		By("Patch shoot status")
		patch := client.MergeFrom(shoot.DeepCopy())
		shoot.Status.TechnicalID = seedNamespace.Name
		shoot.Status.LastOperation = lastOperation
		Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

		By("Ensure manager has observed status patch")
		Eventually(func(g Gomega) string {
			g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			return shoot.Status.TechnicalID
		}).ShouldNot(BeEmpty())

		DeferCleanup(func() {
			By("Delete Shoot")
			Expect(testClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))

			By("Ensure Shoot is gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
			}).Should(BeNotFoundError())

			By("Delete ShootState")
			Expect(testClient.Delete(ctx, shootState)).To(Or(Succeed(), BeNotFoundError()))

			By("Ensure ShootState is gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)
			}).Should(BeNotFoundError())
		})
	})

	Context("when no periodic backup should be performed", func() {
		Context("seed name does not match", func() {
			BeforeEach(func() {
				shoot.Spec.SeedName = ptr.To("some-seed-name")
			})

			It("should do nothing", func() {
				By("Ensure no ShootState gets created")
				Consistently(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), &gardencorev1beta1.ShootState{})
				}).Should(BeNotFoundError())
			})
		})

		It("should do nothing when shoot has deletion timestamp", func() {
			By("Ensure ShootState is newly created")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
				g.Expect(shootState.Spec.Gardener).To(BeEmpty())
			}).Should(Succeed())

			By("Create secret for next backup")
			createPersistableSecret()

			By("Add finalizer to prevent Shoot from disappearing too soon")
			patch := client.MergeFrom(shoot.DeepCopy())
			controllerutil.AddFinalizer(shoot, "foo.com/bar")
			Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

			DeferCleanup(func() {
				Expect(controllerutils.RemoveAllFinalizers(ctx, testClient, shoot)).To(Succeed())
			})

			By("Deleting Shoot")
			Expect(testClient.Delete(ctx, shoot)).To(Succeed())

			By("Ensure ShootState is not updated")
			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
				g.Expect(shootState.Spec.Gardener).To(BeEmpty())
			}).Should(Succeed())
		})
	})

	Context("when shoot is not yet created successfully", func() {
		BeforeEach(func() {
			lastOperation = &gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeCreate,
				State: gardencorev1beta1.LastOperationStateProcessing,
			}
		})

		It("should requeue until creation finished and eventually create the ShootState", func() {
			By("Ensure no ShootState gets created")
			Consistently(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), &gardencorev1beta1.ShootState{})
			}).Should(BeNotFoundError())

			By("Mark shoot as 'creation succeeded'")
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), &gardencorev1beta1.ShootState{})
			}).Should(Succeed())
		})
	})

	Context("when shoot is in migration", func() {
		BeforeEach(func() {
			lastOperation = &gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
				State: gardencorev1beta1.LastOperationStateProcessing,
			}
		})

		It("should requeue until migration finished and eventually create the ShootState", func() {
			By("Ensure no ShootState gets created")
			Consistently(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), &gardencorev1beta1.ShootState{})
			}).Should(BeNotFoundError())

			By("Mark shoot as 'restore processing'")
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeRestore
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

			By("Ensure no ShootState gets created")
			Consistently(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), &gardencorev1beta1.ShootState{})
			}).Should(BeNotFoundError())

			By("Mark shoot as 'restore succeeded'")
			patch = client.MergeFrom(shoot.DeepCopy())
			shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), &gardencorev1beta1.ShootState{})
			}).Should(Succeed())
		})
	})

	Context("when ShootState exists already", func() {
		It("should requeue until next periodic backup is due", func() {
			var lastBackup string

			By("Ensure ShootState is newly created")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
				lastBackup = shootState.Annotations["gardener.cloud/timestamp"]
				g.Expect(shootState.Spec.Gardener).To(BeEmpty())
			}).Should(Succeed())

			By("Create secret for next backup")
			createPersistableSecret()

			By("Ensure ShootState is not updated")
			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
				g.Expect(shootState.Annotations).To(HaveKeyWithValue("gardener.cloud/timestamp", lastBackup))
				g.Expect(shootState.Spec.Gardener).To(BeEmpty())
			}).Should(Succeed())

			By("Step clock")
			fakeClock.Step(2 * syncPeriod)

			By("Ensure ShootState gets updated")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
				g.Expect(shootState.Annotations).To(HaveKeyWithValue("gardener.cloud/timestamp", Not(Equal(lastBackup))))
				g.Expect(shootState.Spec.Gardener).To(HaveLen(1))
			}).Should(Succeed())
		})
	})
})
