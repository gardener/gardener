// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"github.com/gardener/gardener/pkg/operator/apis/config"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/garden/care"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
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
		operatorConfig  config.OperatorConfiguration
		gardenClientMap *fakeclientmap.ClientMap
		reconciler      *Reconciler
		garden          *operatorv1alpha1.Garden
		fakeClock       *testclock.FakeClock
	)

	BeforeEach(func() {
		ctx = context.Background()
		logf.IntoContext(ctx, logr.Discard())

		runtimeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()

		operatorConfig = config.OperatorConfiguration{
			Controllers: config.ControllerConfiguration{
				GardenCare: config.GardenCareControllerConfiguration{
					SyncPeriod: &metav1.Duration{Duration: time.Minute},
				},
			},
		}

		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name: gardenName,
			},
		}

		gardenClientMap = fakeclientmap.NewClientMapBuilder().WithRuntimeClientForKey(keys.ForGarden(garden), runtimeClient).Build()

		fakeClock = testclock.NewFakeClock(time.Now())
	})

	Describe("#Care", func() {
		var req reconcile.Request

		BeforeEach(func() {
			req = reconcile.Request{NamespacedName: kubernetesutils.Key(gardenName)}
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
				req = reconcile.Request{NamespacedName: kubernetesutils.Key("some-other-garden")}
				Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{}))
			})
		})

		Context("when health check setup is successful", func() {
			Context("when no conditions are returned", func() {
				BeforeEach(func() {
					DeferCleanup(test.WithVars(&NewHealthCheck,
						healthCheckFunc(func(_ []gardencorev1beta1.Condition) []gardencorev1beta1.Condition { return nil })))
				})

				It("should not set conditions", func() {
					Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedGarden := &operatorv1alpha1.Garden{}
					Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), updatedGarden)).To(Succeed())
					Expect(updatedGarden.Status.Conditions).To(BeEmpty())
				})

				It("should remove conditions", func() {
					gardenSystemComponentsCondition := gardencorev1beta1.Condition{
						Type:   operatorv1alpha1.GardenSystemComponentsHealthy,
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
						&NewHealthCheck, healthCheckFunc(func(cond []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
							conditionsCopy := append(cond[:0:0], cond...)
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
						Type:   operatorv1alpha1.GardenSystemComponentsHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					}

					garden.Status = operatorv1alpha1.GardenStatus{
						Conditions: []gardencorev1beta1.Condition{gardenSystemComponentsCondition},
					}
					Expect(runtimeClient.Update(ctx, garden)).To(Succeed())

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
							Type:   operatorv1alpha1.GardenSystemComponentsHealthy,
							Status: gardencorev1beta1.ConditionTrue,
							Reason: "foo",
						},
					}
					DeferCleanup(test.WithVars(&NewHealthCheck,
						healthCheckFunc(func(cond []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
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

type resultingConditionFunc func(cond []gardencorev1beta1.Condition) []gardencorev1beta1.Condition

func healthCheckFunc(fn resultingConditionFunc) NewHealthCheckFunc {
	return func(*operatorv1alpha1.Garden, client.Client, kubernetes.Interface, clock.Clock, string) HealthCheck {
		return fn
	}
}

func (c resultingConditionFunc) CheckGarden(_ context.Context,
	conditions []gardencorev1beta1.Condition,
	_ map[gardencorev1beta1.ConditionType]time.Duration) []gardencorev1beta1.Condition {
	return c(conditions)
}
