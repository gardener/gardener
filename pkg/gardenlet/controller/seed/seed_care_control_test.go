// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed_test

import (
	"context"
	"time"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/seed"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"
	"github.com/go-logr/logr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Seed Care Control", func() {
	var (
		ctx           context.Context
		gardenClient  client.Client
		careControl   reconcile.Reconciler
		gardenletConf *config.GardenletConfiguration

		seedName string

		seed *gardencorev1beta1.Seed
	)

	BeforeEach(func() {
		ctx = context.Background()
		logf.IntoContext(ctx, logr.Discard())

		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		seedName = "seed"

		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: seedName,
			},
			Spec: gardencorev1beta1.SeedSpec{
				Settings: &gardencorev1beta1.SeedSettings{
					ShootDNS: &gardencorev1beta1.SeedSettingShootDNS{
						Enabled: true,
					},
				},
			},
			Status: gardencorev1beta1.SeedStatus{
				KubernetesVersion: pointer.StringPtr("latest"),
				Gardener: &gardencorev1beta1.Gardener{
					Version: "latest",
				},
			},
		}
	})

	Describe("#Care", func() {
		var (
			careSyncPeriod time.Duration
			req            reconcile.Request
		)

		BeforeEach(func() {
			careSyncPeriod = 1 * time.Minute

			req = reconcile.Request{NamespacedName: kutil.Key(seedName)}

			gardenletConf = &config.GardenletConfiguration{
				SeedConfig: &config.SeedConfig{
					SeedTemplate: gardencore.SeedTemplate{
						ObjectMeta: metav1.ObjectMeta{
							Name: seedName,
						},
					},
				},
				Controllers: &config.GardenletControllerConfiguration{
					SeedCare: &config.SeedCareControllerConfiguration{
						SyncPeriod: &metav1.Duration{Duration: careSyncPeriod},
					},
				},
			}
		})

		JustBeforeEach(func() {
			Expect(gardenClient.Create(ctx, seed)).To(Succeed())
		})

		Context("when health check setup is broken", func() {
			var clientMapBuilder *fakeclientmap.ClientMapBuilder

			JustBeforeEach(func() {
				gardenClientSet := fakeclientset.NewClientSetBuilder().
					WithClient(gardenClient).
					Build()
				clientMapBuilder.WithClientSetForKey(keys.ForGarden(), gardenClientSet)
			})

			BeforeEach(func() {
				clientMapBuilder = fakeclientmap.NewClientMapBuilder()
			})

			Context("when seed client is not available", func() {
				It("should report a setup failure", func() {
					careControl = NewCareReconciler(clientMapBuilder.Build(), *gardenletConf.Controllers.SeedCare)
					Expect(careControl.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedSeed := &gardencorev1beta1.Seed{}
					Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(seed), updatedSeed)).To(Succeed())
					Expect(updatedSeed.Status.Conditions).To(consistOfConditionsInUnknownStatus("Precondition failed: seed client cannot be constructed"))
				})
			})
		})

		Context("when health check setup is successful", func() {
			var clientMap clientmap.ClientMap

			JustBeforeEach(func() {
				gardenClientSet := fakeclientset.NewClientSetBuilder().
					WithClient(gardenClient).
					Build()
				seedClientSet := fakeclientset.NewClientSetBuilder().
					Build()
				clientMap = fakeclientmap.NewClientMapBuilder().
					WithClientSetForKey(keys.ForGarden(), gardenClientSet).
					WithClientSetForKey(keys.ForSeedWithName(seedName), seedClientSet).
					Build()
				careControl = NewCareReconciler(clientMap, *gardenletConf.Controllers.SeedCare)
			})

			Context("when no conditions are returned", func() {
				BeforeEach(func() {
					DeferCleanup(test.WithVars(&NewHealthCheck,
						healthCheckFunc(func(_ []gardencorev1beta1.Condition) []gardencorev1beta1.Condition { return nil })))
				})
				It("should not set conditions", func() {
					Expect(careControl.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

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

					Expect(careControl.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedSeed := &gardencorev1beta1.Seed{}
					Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(seed), updatedSeed)).To(Succeed())
					Expect(updatedSeed.Status.Conditions).To(BeEmpty())
				})
			})

			Context("when conditions are returned unchanged", func() {
				BeforeEach(func() {
					DeferCleanup(test.WithVars(&NewHealthCheck,
						healthCheckFunc(func(cond []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
							conditionsCopy := append(cond[:0:0], cond...)
							return conditionsCopy
						})))
				})

				It("should not set conditions", func() {
					Expect(careControl.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedSeed := &gardencorev1beta1.Seed{}
					Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(seed), updatedSeed)).To(Succeed())
					Expect(updatedSeed.Status.Conditions).To(BeEmpty())
				})
				It("should not amend existing conditions", func() {
					apiServerCondition := gardencorev1beta1.Condition{
						Type:   gardencorev1beta1.ShootAPIServerAvailable,
						Status: gardencorev1beta1.ConditionTrue,
					}

					seed.Status = gardencorev1beta1.SeedStatus{
						Conditions: []gardencorev1beta1.Condition{apiServerCondition},
					}
					Expect(gardenClient.Update(ctx, seed)).To(Succeed())

					Expect(careControl.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedSeed := &gardencorev1beta1.Seed{}
					Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(seed), updatedSeed)).To(Succeed())
					Expect(updatedSeed.Status.Conditions).To(ConsistOf(apiServerCondition))
				})
			})

			Context("when conditions are changed", func() {
				var (
					conditions []gardencorev1beta1.Condition
				)

				BeforeEach(func() {
					conditions = []gardencorev1beta1.Condition{
						{
							Type:   gardencorev1beta1.SeedSystemComponentsHealthy,
							Status: gardencorev1beta1.ConditionTrue,
							Reason: "foo",
						},
					}
					DeferCleanup(test.WithVars(&NewHealthCheck,
						healthCheckFunc(func(cond []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
							return conditions
						})))
				})
				It("should update shoot conditions", func() {
					Expect(careControl.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedSeed := &gardencorev1beta1.Seed{}
					Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(seed), updatedSeed)).To(Succeed())
					Expect(updatedSeed.Status.Conditions).To(ConsistOf(conditions))
				})
			})
		})
	})
})

type resultingConditionFunc func(cond []gardencorev1beta1.Condition) []gardencorev1beta1.Condition

func healthCheckFunc(fn resultingConditionFunc) NewHealthCheckFunc {
	return func(seed *gardencorev1beta1.Seed, client client.Client, logger logr.Logger) HealthCheck {
		return fn
	}
}

func (c resultingConditionFunc) CheckSeed(_ context.Context,
	seed *seedpkg.Seed,
	conditions []gardencorev1beta1.Condition,
	thresholdMappings map[gardencorev1beta1.ConditionType]time.Duration) []gardencorev1beta1.Condition {
	return c(conditions)
}

func consistOfConditionsInUnknownStatus(message string) types.GomegaMatcher {
	return ConsistOf(
		MatchFields(IgnoreExtras, Fields{
			"Type":    Equal(gardencorev1beta1.SeedSystemComponentsHealthy),
			"Status":  Equal(gardencorev1beta1.ConditionUnknown),
			"Message": Equal(message),
		}),
	)
}

// failingPatchClient returns fake errors for patch operations for testing purposes
type failingPatchClient struct {
	err error
	client.Client
}

func (c failingPatchClient) Patch(context.Context, client.Object, client.Patch, ...client.PatchOption) error {
	return c.err
}
