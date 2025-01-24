// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/seed/care"
	"github.com/gardener/gardener/pkg/utils/test"
)

const (
	seedName       = "seed"
	careSyncPeriod = 1 * time.Minute
)

var _ = Describe("Seed Care Control", func() {
	var (
		ctx              context.Context
		gardenClient     client.Client
		seedClient       client.Client
		reconciler       *Reconciler
		controllerConfig gardenletconfigv1alpha1.SeedCareControllerConfiguration
		seed             *gardencorev1beta1.Seed
		fakeClock        *testclock.FakeClock
	)

	BeforeEach(func() {
		ctx = context.Background()
		logf.IntoContext(ctx, logr.Discard())

		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: seedName,
			},
		}

		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithStatusSubresource(&gardencorev1beta1.Seed{}).Build()
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		fakeClock = testclock.NewFakeClock(time.Now())
	})

	Describe("#Care", func() {
		var req reconcile.Request

		BeforeEach(func() {
			req = reconcile.Request{NamespacedName: client.ObjectKey{Name: seedName}}

			controllerConfig = gardenletconfigv1alpha1.SeedCareControllerConfiguration{
				SyncPeriod: &metav1.Duration{Duration: careSyncPeriod},
			}
		})

		JustBeforeEach(func() {
			Expect(gardenClient.Create(ctx, seed)).To(Succeed())
		})

		Context("when seed no longer exists", func() {
			It("should stop reconciling and not requeue", func() {
				reconciler = &Reconciler{GardenClient: gardenClient, SeedClient: seedClient, Config: controllerConfig, Clock: fakeClock}

				req = reconcile.Request{NamespacedName: client.ObjectKey{Name: "some-other-seed"}}
				Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{}))
			})
		})

		Context("when health check setup is successful", func() {
			JustBeforeEach(func() {
				reconciler = &Reconciler{GardenClient: gardenClient, SeedClient: seedClient, Config: controllerConfig, Clock: fakeClock}
			})

			Context("when no conditions are returned", func() {
				BeforeEach(func() {
					DeferCleanup(test.WithVars(&NewHealthCheck,
						healthCheckFunc(func(_ SeedConditions) []gardencorev1beta1.Condition { return nil })))
				})

				It("should not set conditions", func() {
					Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedSeed := &gardencorev1beta1.Seed{}
					Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(seed), updatedSeed)).To(Succeed())
					Expect(updatedSeed.Status.Conditions).To(BeEmpty())
				})

				It("should remove conditions", func() {
					seedSystemComponentsCondition := gardencorev1beta1.Condition{
						Type:   gardencorev1beta1.SeedSystemComponentsHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					}

					seed.Status = gardencorev1beta1.SeedStatus{
						Conditions: []gardencorev1beta1.Condition{seedSystemComponentsCondition},
					}
					Expect(gardenClient.Update(ctx, seed)).To(Succeed())

					Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedSeed := &gardencorev1beta1.Seed{}
					Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(seed), updatedSeed)).To(Succeed())
					Expect(updatedSeed.Status.Conditions).To(BeEmpty())
				})
			})

			Context("when conditions are returned unchanged", func() {
				BeforeEach(func() {
					DeferCleanup(test.WithVars(
						&NewHealthCheck, healthCheckFunc(func(cond SeedConditions) []gardencorev1beta1.Condition {
							conditions := cond.ConvertToSlice()
							conditionsCopy := append(conditions[:0:0], conditions...)
							return conditionsCopy
						}),
					))
				})

				It("should not set conditions", func() {
					Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedSeed := &gardencorev1beta1.Seed{}
					Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(seed), updatedSeed)).To(Succeed())
					Expect(updatedSeed.Status.Conditions).To(BeEmpty())
				})

				It("should not amend existing conditions", func() {
					seedSystemComponentsCondition := gardencorev1beta1.Condition{
						Type:   gardencorev1beta1.SeedSystemComponentsHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					}

					seed.Status = gardencorev1beta1.SeedStatus{
						Conditions: []gardencorev1beta1.Condition{seedSystemComponentsCondition},
					}
					Expect(gardenClient.Status().Update(ctx, seed)).To(Succeed())

					Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedSeed := &gardencorev1beta1.Seed{}
					Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(seed), updatedSeed)).To(Succeed())
					Expect(updatedSeed.Status.Conditions).To(ConsistOf(seedSystemComponentsCondition))
				})
			})

			Context("when conditions are changed", func() {
				var conditions []gardencorev1beta1.Condition

				BeforeEach(func() {
					conditions = []gardencorev1beta1.Condition{
						{
							Type:   gardencorev1beta1.SeedSystemComponentsHealthy,
							Status: gardencorev1beta1.ConditionTrue,
							Reason: "foo",
						},
					}
					DeferCleanup(test.WithVars(&NewHealthCheck,
						healthCheckFunc(func(_ SeedConditions) []gardencorev1beta1.Condition {
							return conditions
						})))
				})

				It("should update shoot conditions", func() {
					Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedSeed := &gardencorev1beta1.Seed{}
					Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(seed), updatedSeed)).To(Succeed())
					Expect(updatedSeed.Status.Conditions).To(ConsistOf(conditions))
				})
			})
		})
	})
})

type resultingConditionFunc func(conditions SeedConditions) []gardencorev1beta1.Condition

func (c resultingConditionFunc) Check(_ context.Context, conditions SeedConditions) []gardencorev1beta1.Condition {
	return c(conditions)
}

func healthCheckFunc(fn resultingConditionFunc) NewHealthCheckFunc {
	return func(*gardencorev1beta1.Seed, client.Client, clock.Clock, *string, map[gardencorev1beta1.ConditionType]time.Duration) HealthCheck {
		return fn
	}
}
