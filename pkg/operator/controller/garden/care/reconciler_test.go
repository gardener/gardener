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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/operator/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/garden/care"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health/checker"
	"github.com/gardener/gardener/pkg/utils/test"
)

const (
	gardenName     = "garden"
	careSyncPeriod = 1 * time.Minute
)

var _ = Describe("Garden Care Control", func() {
	var (
		ctx             context.Context
		runtimeClient   client.WithWatch
		operatorConfig  operatorconfigv1alpha1.OperatorConfiguration
		gardenClientMap *fakeclientmap.ClientMap
		reconciler      *Reconciler
		garden          *operatorv1alpha1.Garden
		fakeClock       *testclock.FakeClock
	)

	BeforeEach(func() {
		ctx = context.Background()
		logf.IntoContext(ctx, logr.Discard())

		operatorConfig = operatorconfigv1alpha1.OperatorConfiguration{
			Controllers: operatorconfigv1alpha1.ControllerConfiguration{
				GardenCare: operatorconfigv1alpha1.GardenCareControllerConfiguration{
					SyncPeriod: &metav1.Duration{Duration: time.Minute},
				},
			},
		}

		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name: gardenName,
			},
		}

		runtimeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).WithStatusSubresource(&operatorv1alpha1.Garden{}).Build()
		gardenClientMap = fakeclientmap.NewClientMapBuilder().WithRuntimeClientForKey(keys.ForGarden(garden), runtimeClient, nil).Build()

		fakeClock = testclock.NewFakeClock(time.Now())
	})

	Describe("#Care", func() {
		var req reconcile.Request

		BeforeEach(func() {
			req = reconcile.Request{NamespacedName: client.ObjectKey{Name: gardenName}}
		})

		JustBeforeEach(func() {
			Expect(runtimeClient.Create(ctx, garden)).To(Succeed())
			reconciler = &Reconciler{
				RuntimeClient:   runtimeClient,
				Config:          operatorConfig,
				GardenClientMap: gardenClientMap,
				Clock:           fakeClock,
			}
		})

		Context("when garden no longer exists", func() {
			It("should stop reconciling and not requeue", func() {
				req = reconcile.Request{NamespacedName: client.ObjectKey{Name: "some-other-garden"}}
				Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{}))
			})
		})

		Context("when health check setup is successful", func() {
			Context("when no conditions are returned", func() {
				BeforeEach(func() {
					DeferCleanup(test.WithVars(&NewHealthCheck,
						healthCheckFunc(func(_ GardenConditions) []gardencorev1beta1.Condition {
							return nil
						})))
				})

				It("should not set conditions", func() {
					Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedGarden := &operatorv1alpha1.Garden{}
					Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), updatedGarden)).To(Succeed())
					Expect(updatedGarden.Status.Conditions).To(BeEmpty())
				})

				It("should remove conditions", func() {
					gardenSystemComponentsCondition := gardencorev1beta1.Condition{
						Type:   operatorv1alpha1.RuntimeComponentsHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					}

					garden.Status = operatorv1alpha1.GardenStatus{
						Conditions: []gardencorev1beta1.Condition{gardenSystemComponentsCondition},
					}
					Expect(runtimeClient.Update(ctx, garden)).To(Succeed())

					Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedGarden := &operatorv1alpha1.Garden{}
					Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), updatedGarden)).To(Succeed())
					Expect(updatedGarden.Status.Conditions).To(BeEmpty())
				})
			})

			Context("when conditions are returned unchanged", func() {
				BeforeEach(func() {
					DeferCleanup(test.WithVars(
						&NewHealthCheck, healthCheckFunc(func(cond GardenConditions) []gardencorev1beta1.Condition {
							conditions := cond.ConvertToSlice()
							conditionsCopy := append(conditions[:0:0], conditions...)
							return conditionsCopy
						}),
					))
				})

				It("should not set conditions", func() {
					Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedGarden := &operatorv1alpha1.Garden{}
					Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), updatedGarden)).To(Succeed())
					Expect(updatedGarden.Status.Conditions).To(BeEmpty())
				})

				It("should not amend existing conditions", func() {
					gardenSystemComponentsCondition := gardencorev1beta1.Condition{
						Type:   operatorv1alpha1.RuntimeComponentsHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					}

					garden.Status = operatorv1alpha1.GardenStatus{
						Conditions: []gardencorev1beta1.Condition{gardenSystemComponentsCondition},
					}
					Expect(runtimeClient.Status().Update(ctx, garden)).To(Succeed())

					Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedGarden := &operatorv1alpha1.Garden{}
					Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), updatedGarden)).To(Succeed())
					Expect(updatedGarden.Status.Conditions).To(ConsistOf(gardenSystemComponentsCondition))
				})
			})

			Context("when conditions are changed", func() {
				var conditions []gardencorev1beta1.Condition

				BeforeEach(func() {
					conditions = []gardencorev1beta1.Condition{
						{
							Type:   operatorv1alpha1.RuntimeComponentsHealthy,
							Status: gardencorev1beta1.ConditionTrue,
							Reason: "foo",
						},
					}
					DeferCleanup(test.WithVars(&NewHealthCheck,
						healthCheckFunc(func(_ GardenConditions) []gardencorev1beta1.Condition {
							return conditions
						})))
				})

				It("should update garden conditions", func() {
					Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedGarden := &operatorv1alpha1.Garden{}
					Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), updatedGarden)).To(Succeed())
					Expect(updatedGarden.Status.Conditions).To(ConsistOf(conditions))
				})
			})
		})

		Describe("kstatus condition preservation", func() {
			// The garden care controller must not touch Reconciling or Stalled conditions because those are owned by the garden reconciler.
			It("preserves Reconciling=True added during a care reconcile", func() {
				reconcilingCondition := gardencorev1beta1.Condition{
					Type:               gardencorev1beta1.ConditionReconciling,
					Status:             gardencorev1beta1.ConditionTrue,
					Reason:             "ReconcileInProgress",
					Message:            "The resource is currently being reconciled.",
					LastTransitionTime: metav1.NewTime(fakeClock.Now().Truncate(time.Second)),
					LastUpdateTime:     metav1.NewTime(fakeClock.Now().Truncate(time.Second)),
				}
				healthCondition := gardencorev1beta1.Condition{
					Type:   operatorv1alpha1.RuntimeComponentsHealthy,
					Status: gardencorev1beta1.ConditionTrue,
					Reason: "RuntimeComponentsRunning",
				}

				DeferCleanup(test.WithVars(&NewHealthCheck,
					healthCheckFunc(func(_ GardenConditions) []gardencorev1beta1.Condition {
						defer GinkgoRecover()

						// Simulate the garden reconciler adding Reconciling after care has read the Garden.
						current := &operatorv1alpha1.Garden{}
						Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), current)).To(Succeed())
						patch := client.MergeFrom(current.DeepCopy())
						current.Status.Conditions = append(current.Status.Conditions, reconcilingCondition)
						Expect(runtimeClient.Status().Patch(ctx, current, patch)).To(Succeed())

						// Return changed health conditions to force care to patch status.conditions.
						return []gardencorev1beta1.Condition{healthCondition}
					}),
				))

				Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

				updated := &operatorv1alpha1.Garden{}
				Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: gardenName}, updated)).To(Succeed())
				Expect(updated.Status.Conditions).To(ContainElement(healthCondition))
				Expect(updated.Status.Conditions).To(ContainElement(reconcilingCondition), "care controller must preserve Reconciling condition set by the reconciler")
			})

			It("preserves Stalled=True through a care reconcile", func() {
				patch := client.MergeFrom(garden.DeepCopy())
				garden.Status.Conditions = []gardencorev1beta1.Condition{
					{
						Type:               gardencorev1beta1.ConditionStalled,
						Status:             gardencorev1beta1.ConditionTrue,
						Reason:             "Error",
						Message:            "reconcile error",
						LastTransitionTime: metav1.NewTime(fakeClock.Now()),
						LastUpdateTime:     metav1.NewTime(fakeClock.Now()),
					},
				}
				Expect(runtimeClient.Status().Patch(ctx, garden, patch)).To(Succeed())

				DeferCleanup(test.WithVars(&NewHealthCheck,
					healthCheckFunc(func(cond GardenConditions) []gardencorev1beta1.Condition {
						return cond.ConvertToSlice()
					}),
				))

				Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

				updated := &operatorv1alpha1.Garden{}
				Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: gardenName}, updated)).To(Succeed())
				Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(gardencorev1beta1.ConditionStalled),
					"Status": Equal(gardencorev1beta1.ConditionTrue),
				})), "care controller must preserve Stalled condition set by the reconciler")
			})

			It("returns a conflict when conditions change between the latest read and the patch", func() {
				reconcilingCondition := gardencorev1beta1.Condition{
					Type:   gardencorev1beta1.ConditionReconciling,
					Status: gardencorev1beta1.ConditionTrue,
					Reason: "ReconcileInProgress",
				}
				healthCondition := gardencorev1beta1.Condition{
					Type:   operatorv1alpha1.RuntimeComponentsHealthy,
					Status: gardencorev1beta1.ConditionTrue,
					Reason: "RuntimeComponentsRunning",
				}
				DeferCleanup(test.WithVars(&NewHealthCheck,
					healthCheckFunc(func(_ GardenConditions) []gardencorev1beta1.Condition {
						return []gardencorev1beta1.Condition{healthCondition}
					}),
				))

				concurrentUpdate := true
				reconciler.RuntimeClient = interceptor.NewClient(runtimeClient, interceptor.Funcs{
					SubResourcePatch: func(ctx context.Context, c client.Client, subResource string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
						if concurrentUpdate {
							concurrentUpdate = false
							current := &operatorv1alpha1.Garden{}
							Expect(c.Get(ctx, client.ObjectKeyFromObject(garden), current)).To(Succeed())
							current.Status.Conditions = append(current.Status.Conditions, reconcilingCondition)
							Expect(c.Status().Update(ctx, current)).To(Succeed())
						}
						return c.SubResource(subResource).Patch(ctx, obj, patch, opts...)
					},
				})

				result, err := reconciler.Reconcile(ctx, req)
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(apierrors.IsConflict, "a conflict error"))

				updated := &operatorv1alpha1.Garden{}
				Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), updated)).To(Succeed())
				Expect(updated.Status.Conditions).To(ConsistOf(reconcilingCondition))

				By("reconciling again with the latest status")
				Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))
				Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), updated)).To(Succeed())
				Expect(updated.Status.Conditions).To(ConsistOf(reconcilingCondition, healthCondition))
			})
		})
	})
})

type resultingConditionFunc func(cond GardenConditions) []gardencorev1beta1.Condition

func (c resultingConditionFunc) Check(_ context.Context, conditions GardenConditions) []gardencorev1beta1.Condition {
	return c(conditions)
}

func healthCheckFunc(fn resultingConditionFunc) NewHealthCheckFunc {
	return func(*operatorv1alpha1.Garden, client.Client, kubernetes.Interface, clock.Clock, map[gardencorev1beta1.ConditionType]time.Duration, string, *checker.HealthChecker) HealthCheck {
		return fn
	}
}
