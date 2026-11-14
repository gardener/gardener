// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package seed_test

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
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	seedcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/seed/seed"
)

var _ = Describe("Seed reconciler kstatus conditions", func() {
	var (
		ctx           context.Context
		gardenClient  client.Client
		reconciler    *seedcontroller.Reconciler
		seed          *gardencorev1beta1.Seed
		fakeClock     *testclock.FakeClock
		statusUpdates []gardencorev1beta1.SeedStatus
	)

	BeforeEach(func() {
		ctx = context.Background()
		logf.IntoContext(ctx, logr.Discard())

		fakeClock = testclock.NewFakeClock(time.Now())
		statusUpdates = nil

		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-seed",
				Generation: 3,
			},
		}

		gardenClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.GardenScheme).
			WithStatusSubresource(&gardencorev1beta1.Seed{}).
			WithInterceptorFuncs(interceptor.Funcs{
				SubResourceUpdate: func(ctx context.Context, c client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
					if subResourceName == "status" {
						if s, ok := obj.(*gardencorev1beta1.Seed); ok {
							statusUpdates = append(statusUpdates, *s.Status.DeepCopy())
						}
					}
					return c.SubResource(subResourceName).Update(ctx, obj, opts...)
				},
			}).
			Build()

		reconciler = &seedcontroller.Reconciler{
			GardenClient: gardenClient,
			SeedClientSet: fakekubernetes.NewClientSetBuilder().
				WithClient(fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()).
				WithVersion("1.30.0").
				Build(),
			Config: gardenletconfigv1alpha1.GardenletConfiguration{
				Controllers: &gardenletconfigv1alpha1.GardenletControllerConfiguration{
					Seed: &gardenletconfigv1alpha1.SeedControllerConfiguration{
						SyncPeriod: &metav1.Duration{Duration: time.Minute},
					},
				},
				SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
					SeedTemplate: gardencorev1beta1.SeedTemplate{
						ObjectMeta: metav1.ObjectMeta{Name: "test-seed"},
					},
				},
			},
			Identity:        &gardencorev1beta1.Gardener{Version: "1.2.3"},
			GardenNamespace: "garden-test-seed",
			Clock:           fakeClock,
		}

		Expect(gardenClient.Create(ctx, seed)).To(Succeed())
	})

	Describe("on reconciliation start it errors", func() {
		It("sets Reconciling=True then transitions to Stalled=True; ObservedGeneration is bumped", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKey{Name: seed.Name},
			})
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			updated := &gardencorev1beta1.Seed{}
			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(seed), updated)).To(Succeed())

			By("ObservedGeneration must equal Generation (set during updateStatusOperationStart)")
			Expect(updated.Status.ObservedGeneration).To(Equal(seed.Generation))

			By("Reconciling condition must be absent (removed by updateStatusOperationError)")
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionReconciling),
			})))

			By("Stalled condition must be True (set by updateStatusOperationError)")
			Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(gardencorev1beta1.ConditionStalled),
				"Status": Equal(gardencorev1beta1.ConditionTrue),
			})))
		})
	})

	Describe("kstatus condition isolation: stale Stalled is replaced on next error", func() {
		It("a stale Stalled condition is replaced when a new error occurs", func() {
			patch := client.MergeFrom(seed.DeepCopy())
			seed.Status.Conditions = []gardencorev1beta1.Condition{
				{
					Type:               gardencorev1beta1.ConditionStalled,
					Status:             gardencorev1beta1.ConditionTrue,
					Reason:             "Error",
					Message:            "old error from previous reconcile",
					LastTransitionTime: metav1.NewTime(fakeClock.Now()),
					LastUpdateTime:     metav1.NewTime(fakeClock.Now()),
				},
			}
			Expect(gardenClient.Status().Patch(ctx, seed, patch)).To(Succeed())

			_, _ = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKey{Name: seed.Name},
			})

			updated := &gardencorev1beta1.Seed{}
			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(seed), updated)).To(Succeed())

			var stalledCond *gardencorev1beta1.Condition
			for i := range updated.Status.Conditions {
				if updated.Status.Conditions[i].Type == gardencorev1beta1.ConditionStalled {
					stalledCond = &updated.Status.Conditions[i]
					break
				}
			}
			Expect(stalledCond).NotTo(BeNil())
			Expect(stalledCond.Message).NotTo(Equal("old error from previous reconcile"),
				"Stalled must have been cleared by start and re-set with the current error")
		})
	})

	Describe("when previously Stalled=True", func() {
		It("removes the Stalled condition and sets Reconciling=True upon a subsequent reconciliation run", func() {
			patch := client.MergeFrom(seed.DeepCopy())
			seed.Status.Conditions = []gardencorev1beta1.Condition{
				{
					Type:               gardencorev1beta1.ConditionStalled,
					Status:             gardencorev1beta1.ConditionTrue,
					Reason:             "Error",
					Message:            "previous error",
					LastTransitionTime: metav1.NewTime(fakeClock.Now()),
					LastUpdateTime:     metav1.NewTime(fakeClock.Now()),
				},
			}
			Expect(gardenClient.Status().Patch(ctx, seed, patch)).To(Succeed())

			_, _ = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKey{Name: seed.Name},
			})

			Expect(statusUpdates).NotTo(BeEmpty())
			initialStatus := statusUpdates[0]

			By("ObservedGeneration must equal Generation")
			Expect(initialStatus.ObservedGeneration).To(Equal(seed.Generation))

			By("Stalled must be absent upon reconciliation run")
			Expect(initialStatus.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionStalled),
			})))

			By("Reconciling must be True upon reconciliation run")
			Expect(initialStatus.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(gardencorev1beta1.ConditionReconciling),
				"Status": Equal(gardencorev1beta1.ConditionTrue),
			})))
		})
	})
})
