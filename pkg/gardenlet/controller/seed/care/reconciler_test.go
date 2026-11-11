// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/seed/care"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health/checker"
	"github.com/gardener/gardener/pkg/utils/test"
)

const (
	seedName       = "seed"
	careSyncPeriod = 1 * time.Minute
)

var _ = Describe("Seed Care Control", func() {
	var (
		ctx              context.Context
		gardenClient     client.WithWatch
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

	Describe("#Reconcile", func() {
		Describe("emergency switch annotation condition", func() {
			var req reconcile.Request

			BeforeEach(func() {
				req = reconcile.Request{NamespacedName: client.ObjectKey{Name: seedName}}

				controllerConfig = gardenletconfigv1alpha1.SeedCareControllerConfiguration{
					SyncPeriod: &metav1.Duration{Duration: careSyncPeriod},
				}
			})

			JustBeforeEach(func() {
				reconciler = &Reconciler{GardenClient: gardenClient, SeedClient: seedClient, Config: controllerConfig, Clock: fakeClock}
				Expect(gardenClient.Create(ctx, seed)).To(Succeed())
			})

			It("should add a condition for a set annotation", func() {
				seed.Annotations = map[string]string{
					v1beta1constants.AnnotationEmergencyStopShootReconciliations: "true",
				}
				Expect(gardenClient.Update(ctx, seed)).To(Succeed())

				Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

				updatedSeed := &gardencorev1beta1.Seed{}
				Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(seed), updatedSeed)).To(Succeed())
				Expect(updatedSeed.Status.Conditions).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Type": BeEquivalentTo("SeedSystemComponentsHealthy"),
					}), MatchFields(IgnoreExtras, Fields{
						"Type":    BeEquivalentTo("EmergencyStopShootReconciliations"),
						"Status":  BeEquivalentTo("True"),
						"Reason":  Equal("EmergencyStopShootReconciliations"),
						"Message": Equal("Reconciliations of Shoots managed by this Seed cluster are currently disabled by annotation."),
					}),
				))
			})

			It("should remove a previously set condition for a removed annotation", func() {
				seed.Status.Conditions = []gardencorev1beta1.Condition{
					{
						Type:    gardencorev1beta1.SeedEmergencyStopShootReconciliations,
						Status:  gardencorev1beta1.ConditionFalse,
						Reason:  string(gardencorev1beta1.SeedEmergencyStopShootReconciliations),
						Message: "Reconciliations of Shoots managed by this Seed cluster are currently disabled by annotation.",
					},
				}
				Expect(gardenClient.Status().Update(ctx, seed)).To(Succeed())

				Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

				updatedSeed := &gardencorev1beta1.Seed{}
				Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(seed), updatedSeed)).To(Succeed())
				Expect(updatedSeed.Status.Conditions).To(ConsistOf(MatchFields(IgnoreExtras, Fields{
					"Type": BeEquivalentTo("SeedSystemComponentsHealthy"),
				})))
			})
		})

		Describe("kstatus condition preservation", func() {
			// The care controller must not touch Reconciling or Stalled conditions because those are owned by the seed reconciler.
			BeforeEach(func() {
				seed = &gardencorev1beta1.Seed{
					ObjectMeta: metav1.ObjectMeta{Name: seedName},
				}
				Expect(gardenClient.Create(ctx, seed)).To(Succeed())
				Expect(gardenClient.Status().Update(ctx, seed)).To(Succeed())

				reconciler = &Reconciler{
					GardenClient: gardenClient,
					SeedClient:   fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build(),
					Config: gardenletconfigv1alpha1.SeedCareControllerConfiguration{
						SyncPeriod: &metav1.Duration{Duration: careSyncPeriod},
					},
					Clock: fakeClock,
				}
			})

			DescribeTable("preserves pre-existing kstatus conditions updated by the seed reconciler during health checks", func(conditionType gardencorev1beta1.ConditionType) {
				kstatusCondition := gardencorev1beta1.Condition{
					Type:   conditionType,
					Status: gardencorev1beta1.ConditionTrue,
					Reason: "ConcurrentUpdate",
				}
				seed.Status.Conditions = []gardencorev1beta1.Condition{{
					Type:   conditionType,
					Status: gardencorev1beta1.ConditionFalse,
					Reason: "BeforeConcurrentUpdate",
				}}
				Expect(gardenClient.Status().Update(ctx, seed)).To(Succeed())
				healthCondition := gardencorev1beta1.Condition{
					Type:   gardencorev1beta1.SeedSystemComponentsHealthy,
					Status: gardencorev1beta1.ConditionTrue,
					Reason: "AllHealthy",
				}
				DeferCleanup(test.WithVars(&NewHealthCheck,
					healthCheckFunc(func(_ SeedConditions) []gardencorev1beta1.Condition {
						defer GinkgoRecover()

						// The seed reconciler updates its pre-existing condition after care has read the Seed.
						current := &gardencorev1beta1.Seed{}
						Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(seed), current)).To(Succeed())
						patch := client.MergeFrom(current.DeepCopy())
						Expect(current.Status.Conditions).To(HaveLen(1))
						Expect(current.Status.Conditions[0].Type).To(Equal(conditionType))
						current.Status.Conditions[0] = kstatusCondition
						Expect(gardenClient.Status().Patch(ctx, current, patch)).To(Succeed())

						// Return changed health conditions to force care to patch status.conditions.
						return []gardencorev1beta1.Condition{healthCondition}
					}),
				))

				Expect(reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: seedName}})).
					To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

				updated := &gardencorev1beta1.Seed{}
				Expect(gardenClient.Get(ctx, client.ObjectKey{Name: seedName}, updated)).To(Succeed())

				Expect(updated.Status.Conditions).To(ConsistOf(healthCondition, kstatusCondition))
			},
				Entry("Reconciling=True", gardencorev1beta1.ConditionReconciling),
				Entry("Stalled=True", gardencorev1beta1.ConditionStalled),
			)

			DescribeTable("preserves pre-existing kstatus conditions updated by the seed reconciler immediately before the care status patch", func(conditionType gardencorev1beta1.ConditionType) {
				kstatusCondition := gardencorev1beta1.Condition{
					Type:   conditionType,
					Status: gardencorev1beta1.ConditionTrue,
					Reason: "ConcurrentUpdate",
				}
				seed.Status.Conditions = []gardencorev1beta1.Condition{{
					Type:   conditionType,
					Status: gardencorev1beta1.ConditionFalse,
					Reason: "BeforeConcurrentUpdate",
				}}
				Expect(gardenClient.Status().Update(ctx, seed)).To(Succeed())
				healthCondition := gardencorev1beta1.Condition{
					Type:   gardencorev1beta1.SeedSystemComponentsHealthy,
					Status: gardencorev1beta1.ConditionTrue,
					Reason: "AllHealthy",
				}
				DeferCleanup(test.WithVars(&NewHealthCheck,
					healthCheckFunc(func(_ SeedConditions) []gardencorev1beta1.Condition {
						return []gardencorev1beta1.Condition{healthCondition}
					}),
				))

				reconciler.GardenClient = interceptor.NewClient(gardenClient, interceptor.Funcs{
					SubResourcePatch: func(ctx context.Context, c client.Client, subResource string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
						current := &gardencorev1beta1.Seed{}
						Expect(c.Get(ctx, client.ObjectKeyFromObject(seed), current)).To(Succeed())
						Expect(current.Status.Conditions).To(HaveLen(1))
						Expect(current.Status.Conditions[0].Type).To(Equal(conditionType))
						current.Status.Conditions[0] = kstatusCondition
						Expect(c.Status().Update(ctx, current)).To(Succeed())
						return c.SubResource(subResource).Patch(ctx, obj, patch, opts...)
					},
				})

				Expect(reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})).
					To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

				updated := &gardencorev1beta1.Seed{}
				Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(seed), updated)).To(Succeed())
				Expect(updated.Status.Conditions).To(ConsistOf(kstatusCondition, healthCondition))
			},
				Entry("Reconciling=True", gardencorev1beta1.ConditionReconciling),
				Entry("Stalled=True", gardencorev1beta1.ConditionStalled),
			)

			It("preserves Stalled=True when the health check returns conditions", func() {
				patch := client.MergeFrom(seed.DeepCopy())
				seed.Status.Conditions = []gardencorev1beta1.Condition{
					{
						Type:               gardencorev1beta1.ConditionStalled,
						Status:             gardencorev1beta1.ConditionTrue,
						Reason:             "Error",
						Message:            "reconcile error",
						LastTransitionTime: metav1.NewTime(fakeClock.Now()),
						LastUpdateTime:     metav1.NewTime(fakeClock.Now()),
					},
				}
				Expect(gardenClient.Status().Patch(ctx, seed, patch)).To(Succeed())

				DeferCleanup(test.WithVars(&NewHealthCheck,
					healthCheckFunc(func(_ SeedConditions) []gardencorev1beta1.Condition {
						return []gardencorev1beta1.Condition{
							{
								Type:   gardencorev1beta1.SeedSystemComponentsHealthy,
								Status: gardencorev1beta1.ConditionTrue,
								Reason: "AllHealthy",
							},
						}
					}),
				))

				Expect(reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: seedName}})).
					To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

				updated := &gardencorev1beta1.Seed{}
				Expect(gardenClient.Get(ctx, client.ObjectKey{Name: seedName}, updated)).To(Succeed())

				Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(gardencorev1beta1.ConditionStalled),
					"Status": Equal(gardencorev1beta1.ConditionTrue),
				})), "care controller must not remove Stalled condition set by the reconciler")
			})
		})
	})
})

type resultingConditionFunc func(conditions SeedConditions) []gardencorev1beta1.Condition

func (c resultingConditionFunc) Check(_ context.Context, conditions SeedConditions) []gardencorev1beta1.Condition {
	return c(conditions)
}

func healthCheckFunc(fn resultingConditionFunc) NewHealthCheckFunc {
	return func(*gardencorev1beta1.Seed, client.Client, clock.Clock, *string, *checker.HealthChecker) HealthCheck {
		return fn
	}
}
