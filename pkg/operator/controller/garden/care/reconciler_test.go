// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/garden/care"
	"github.com/gardener/gardener/pkg/utils/test"
)

const (
	gardenName     = "garden"
	careSyncPeriod = 1 * time.Minute
)

var _ = Describe("Garden Care Control", func() {
	var (
		ctx             context.Context
		runtimeClient   client.Client
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
						healthCheckFunc(func(_ GardenConditions) []gardencorev1beta1.Condition { return nil })))
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
	})
})

type resultingConditionFunc func(cond GardenConditions) []gardencorev1beta1.Condition

func (c resultingConditionFunc) Check(_ context.Context, conditions GardenConditions) []gardencorev1beta1.Condition {
	return c(conditions)
}

func healthCheckFunc(fn resultingConditionFunc) NewHealthCheckFunc {
	return func(*operatorv1alpha1.Garden, client.Client, kubernetes.Interface, clock.Clock, map[gardencorev1beta1.ConditionType]time.Duration, string) HealthCheck {
		return fn
	}
}
