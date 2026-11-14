// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
)

var _ = Describe("Shoot reconciler kstatus conditions", func() {
	var (
		ctx          context.Context
		gardenClient client.Client
		reconciler   *Reconciler
		shoot        *gardencorev1beta1.Shoot
		fakeClock    *testclock.FakeClock
	)

	BeforeEach(func() {
		ctx = context.Background()
		logf.IntoContext(ctx, logr.Discard())

		fakeClock = testclock.NewFakeClock(time.Now())

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-shoot",
				Namespace:  "garden-project",
				Generation: 7,
			},
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.31.0",
				},
			},
		}

		gardenClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.GardenScheme).
			WithStatusSubresource(&gardencorev1beta1.Shoot{}).
			Build()

		reconciler = &Reconciler{
			GardenClient: gardenClient,
			SeedClientSet: fakekubernetes.NewClientSetBuilder().
				WithClient(fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()).
				WithVersion("1.30.0").
				Build(),
			Config: gardenletconfigv1alpha1.GardenletConfiguration{
				Controllers: &gardenletconfigv1alpha1.GardenletControllerConfiguration{
					Shoot: &gardenletconfigv1alpha1.ShootControllerConfiguration{
						SyncPeriod:    &metav1.Duration{Duration: time.Minute},
						RetryDuration: &metav1.Duration{Duration: 24 * time.Hour},
					},
				},
				SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
					SeedTemplate: gardencorev1beta1.SeedTemplate{
						ObjectMeta: metav1.ObjectMeta{Name: "test-seed"},
					},
				},
			},
			Identity: &gardencorev1beta1.Gardener{Version: "1.2.3"},
			Clock:    fakeClock,
		}

		Expect(gardenClient.Create(ctx, shoot)).To(Succeed())
	})

	Describe("updateShootStatusOperationStart", func() {
		It("sets Reconciling=True and bumps ObservedGeneration atomically", func() {
			Expect(reconciler.updateShootStatusOperationStart(
				ctx, shoot, "shoot--garden-project--test-shoot", gardencorev1beta1.LastOperationTypeReconcile,
			)).To(Succeed())

			updated := &gardencorev1beta1.Shoot{}
			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), updated)).To(Succeed())

			By("ObservedGeneration must equal Generation")
			Expect(updated.Status.ObservedGeneration).To(Equal(shoot.Generation))

			By("Reconciling must be True")
			Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(gardencorev1beta1.ConditionReconciling),
				"Status": Equal(gardencorev1beta1.ConditionTrue),
			})))

			By("Stalled must be absent")
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionStalled),
			})))
		})

		It("clears a pre-existing Stalled condition on start", func() {
			By("Pre-seed Stalled condition")
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.Conditions = []gardencorev1beta1.Condition{
				{
					Type:               gardencorev1beta1.ConditionStalled,
					Status:             gardencorev1beta1.ConditionTrue,
					Reason:             "Error",
					Message:            "previous error",
					LastTransitionTime: metav1.NewTime(fakeClock.Now()),
					LastUpdateTime:     metav1.NewTime(fakeClock.Now()),
				},
			}
			Expect(gardenClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

			Expect(reconciler.updateShootStatusOperationStart(
				ctx, shoot, "shoot--garden-project--test-shoot", gardencorev1beta1.LastOperationTypeReconcile,
			)).To(Succeed())

			updated := &gardencorev1beta1.Shoot{}
			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), updated)).To(Succeed())

			By("Stalled must be absent after start clears it")
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionStalled),
			})))

			By("Reconciling must be True")
			Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(gardencorev1beta1.ConditionReconciling),
				"Status": Equal(gardencorev1beta1.ConditionTrue),
			})))
		})
	})

	Describe("patchShootStatusOperationError", func() {
		BeforeEach(func() {
			Expect(reconciler.updateShootStatusOperationStart(
				ctx, shoot, "shoot--garden-project--test-shoot", gardencorev1beta1.LastOperationTypeReconcile,
			)).To(Succeed())
		})

		It("removes Reconciling and sets Stalled=True on error", func() {
			Expect(reconciler.patchShootStatusOperationError(
				ctx, shoot, "something went wrong",
				gardencorev1beta1.LastOperationTypeReconcile, false,
			)).To(Succeed())

			updated := &gardencorev1beta1.Shoot{}
			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), updated)).To(Succeed())

			By("Reconciling must be absent")
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionReconciling),
			})))

			By("Stalled must be True")
			Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(gardencorev1beta1.ConditionStalled),
				"Status": Equal(gardencorev1beta1.ConditionTrue),
			})))
		})

		It("removes both Reconciling and Stalled on success", func() {
			Expect(reconciler.patchShootStatusOperationSuccess(
				ctx, shoot, gardencorev1beta1.LastOperationTypeReconcile,
			)).To(Succeed())

			updated := &gardencorev1beta1.Shoot{}
			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), updated)).To(Succeed())

			By("Reconciling must be absent")
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionReconciling),
			})))

			By("Stalled must be absent")
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionStalled),
			})))
		})

		It("overwrites a stale Stalled message with the new error on repeated failures", func() {
			By("First error")
			Expect(reconciler.patchShootStatusOperationError(
				ctx, shoot, "first error",
				gardencorev1beta1.LastOperationTypeReconcile, false,
			)).To(Succeed())

			By("Start a new reconcile (clears old Stalled, sets Reconciling=True)")
			Expect(reconciler.updateShootStatusOperationStart(
				ctx, shoot, "shoot--garden-project--test-shoot", gardencorev1beta1.LastOperationTypeReconcile,
			)).To(Succeed())

			By("Second error")
			Expect(reconciler.patchShootStatusOperationError(
				ctx, shoot, "second error",
				gardencorev1beta1.LastOperationTypeReconcile, false,
			)).To(Succeed())

			updated := &gardencorev1beta1.Shoot{}
			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), updated)).To(Succeed())

			stalledCond := findShootCondition(updated.Status.Conditions, gardencorev1beta1.ConditionStalled)
			Expect(stalledCond).NotTo(BeNil())
			Expect(stalledCond.Message).To(ContainSubstring("second error"),
				"Stalled must have the new error message, not the old one")
		})
	})
})

func findShootCondition(conditions []gardencorev1beta1.Condition, condType gardencorev1beta1.ConditionType) *gardencorev1beta1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}
