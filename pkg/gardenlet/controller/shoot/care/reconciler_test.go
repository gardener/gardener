// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"errors"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/care"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/care"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Shoot Care Control", func() {
	var (
		ctx           context.Context
		gardenClient  client.Client
		reconciler    reconcile.Reconciler
		gardenletConf config.GardenletConfiguration
		fakeClock     *testclock.FakeClock

		shootName, shootNamespace, seedName string

		shoot *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		ctx = context.Background()

		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		shootName = "shoot"
		shootNamespace = "project"
		seedName = "seed"

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: shootNamespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: &seedName,
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{
						{Name: "foo"},
					},
				},
			},
		}

		fakeClock = testclock.NewFakeClock(time.Now())
	})

	AfterEach(func() {
		reconciler = nil
	})

	Describe("#Care", func() {
		var (
			careSyncPeriod time.Duration

			gardenSecrets []corev1.Secret
			req           reconcile.Request
		)

		BeforeEach(func() {
			careSyncPeriod = 1 * time.Minute

			gardenSecrets = []corev1.Secret{{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "internal-domain-secret",
					Namespace:   gardenerutils.ComputeGardenNamespace(seedName),
					Annotations: map[string]string{gardenerutils.DNSProvider: "fooDNS", gardenerutils.DNSDomain: "foo.bar"},
					Labels:      map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleInternalDomain},
				},
			}}

			req = reconcile.Request{NamespacedName: kubernetesutils.Key(shootNamespace, shootName)}

			gardenletConf = config.GardenletConfiguration{
				Controllers: &config.GardenletControllerConfiguration{
					ShootCare: &config.ShootCareControllerConfiguration{
						SyncPeriod: &metav1.Duration{Duration: careSyncPeriod},
					},
				},
			}
		})

		JustBeforeEach(func() {
			Expect(gardenClient.Create(ctx, shoot)).To(Succeed())

			for _, secret := range gardenSecrets {
				Expect(gardenClient.Create(ctx, secret.DeepCopy())).To(Succeed())
			}
		})

		Context("when health check setup is broken", func() {
			Context("when operation cannot be created", func() {
				JustBeforeEach(func() {
					fakeErr := errors.New("foo")
					DeferCleanup(test.WithVar(&NewOperation, opFunc(nil, fakeErr)))

					reconciler = &Reconciler{
						GardenClient:  gardenClient,
						SeedClientSet: kubernetesfake.NewClientSet(),
						Config:        gardenletConf,
						Clock:         fakeClock,
						SeedName:      seedName,
					}

					result, err := reconciler.Reconcile(ctx, req)
					Expect(result).To(Equal(reconcile.Result{}))
					Expect(err).To(MatchError(fakeErr))
				})

				Context("shoot with workers", func() {
					It("should report a setup failure", func() {
						updatedShoot := &gardencorev1beta1.Shoot{}
						Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), updatedShoot)).To(Succeed())
						Expect(updatedShoot.Status.Conditions).To(consistOfConditionsInUnknownStatus("Precondition failed: operation could not be initialized", v1beta1helper.IsWorkerless(shoot)))
						Expect(updatedShoot.Status.Constraints).To(consistOfConstraintsInUnknownStatus("Precondition failed: operation could not be initialized"))
					})
				})

				Context("workerless shoot", func() {
					BeforeEach(func() {
						shoot.Spec.Provider.Workers = nil
					})

					It("should report a setup failure", func() {
						updatedShoot := &gardencorev1beta1.Shoot{}
						Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), updatedShoot)).To(Succeed())
						Expect(updatedShoot.Status.Conditions).To(consistOfConditionsInUnknownStatus("Precondition failed: operation could not be initialized", v1beta1helper.IsWorkerless(shoot)))
						Expect(updatedShoot.Status.Constraints).To(consistOfConstraintsInUnknownStatus("Precondition failed: operation could not be initialized"))
					})
				})
			})

			Context("when Garden secrets are incomplete", func() {
				BeforeEach(func() {
					gardenSecrets = nil
				})

				It("should report a setup failure", func() {
					operationFunc := opFunc(nil, errors.New("foo"))
					defer test.WithVars(&NewOperation, operationFunc)()
					reconciler = &Reconciler{
						GardenClient:  gardenClient,
						SeedClientSet: kubernetesfake.NewClientSet(),
						Config:        gardenletConf,
						Clock:         fakeClock,
						SeedName:      seedName,
					}

					_, err := reconciler.Reconcile(ctx, req)
					Expect(err).To(MatchError("error reading Garden secrets: need an internal domain secret but found none"))
				})
			})
		})

		Context("when health check setup is successful", func() {
			var (
				shootClientMap clientmap.ClientMap
				managedSeed    *seedmanagementv1alpha1.ManagedSeed
				operationFunc  NewOperationFunc
			)

			JustBeforeEach(func() {
				shootClientMap = fakeclientmap.NewClientMapBuilder().Build()

				op := &operation.Operation{
					GardenClient:  gardenClient,
					SeedClientSet: kubernetesfake.NewClientSetBuilder().Build(),
					ManagedSeed:   managedSeed,
					Shoot:         &shootpkg.Shoot{},
					Logger:        logr.Discard(),
				}
				op.Shoot.SetInfo(shoot)
				operationFunc = opFunc(op, nil)

				DeferCleanup(test.WithVars(
					&NewOperation, operationFunc,
					&NewGarbageCollector, nopGarbageCollectorFunc(),
				))
				reconciler = &Reconciler{
					GardenClient:   gardenClient,
					SeedClientSet:  kubernetesfake.NewClientSet(),
					ShootClientMap: shootClientMap,
					Config:         gardenletConf,
					Clock:          fakeClock,
					SeedName:       seedName,
				}
			})

			AfterEach(func() {
				shoot = nil
			})

			Context("when no conditions / constraints are returned", func() {
				BeforeEach(func() {
					DeferCleanup(test.WithVars(
						&NewHealthCheck, healthCheckFunc(func(_ []gardencorev1beta1.Condition) []gardencorev1beta1.Condition { return nil }),
						&NewConstraintCheck, constraintCheckFunc(func(_ []gardencorev1beta1.Condition) []gardencorev1beta1.Condition { return nil }),
					))
				})

				It("should not set conditions / constraints", func() {
					Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedShoot := &gardencorev1beta1.Shoot{}
					Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), updatedShoot)).To(Succeed())
					Expect(updatedShoot.Status.Conditions).To(BeEmpty())
					Expect(updatedShoot.Status.Constraints).To(BeEmpty())
				})

				It("should remove conditions / constraints", func() {
					apiServerCondition := gardencorev1beta1.Condition{
						Type:   gardencorev1beta1.ShootAPIServerAvailable,
						Status: gardencorev1beta1.ConditionTrue,
					}

					hibernationConstraint := gardencorev1beta1.Condition{
						Type:   gardencorev1beta1.ShootHibernationPossible,
						Status: gardencorev1beta1.ConditionFalse,
					}

					shoot.Status = gardencorev1beta1.ShootStatus{
						Conditions:  []gardencorev1beta1.Condition{apiServerCondition},
						Constraints: []gardencorev1beta1.Condition{hibernationConstraint},
					}
					Expect(gardenClient.Update(ctx, shoot)).To(Succeed())

					Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedShoot := &gardencorev1beta1.Shoot{}
					Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), updatedShoot)).To(Succeed())
					Expect(updatedShoot.Status.Conditions).To(BeEmpty())
					Expect(updatedShoot.Status.Constraints).To(BeEmpty())
				})
			})

			Context("when conditions / constraints are returned unchanged", func() {
				BeforeEach(func() {
					DeferCleanup(test.WithVars(
						&NewHealthCheck, healthCheckFunc(func(cond []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
							conditionsCopy := append(cond[:0:0], cond...)
							return conditionsCopy
						}),
						&NewConstraintCheck, constraintCheckFunc(func(constr []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
							constraintsCopy := append(constr[:0:0], constr...)
							return constraintsCopy
						}),
					))
				})

				It("should not set conditions / constraints", func() {
					Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedShoot := &gardencorev1beta1.Shoot{}
					Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), updatedShoot)).To(Succeed())
					Expect(updatedShoot.Status.Conditions).To(BeEmpty())
				})

				It("should not amend existing conditions / constraints", func() {
					apiServerCondition := gardencorev1beta1.Condition{
						Type:   gardencorev1beta1.ShootAPIServerAvailable,
						Status: gardencorev1beta1.ConditionTrue,
					}

					hibernationConstraint := gardencorev1beta1.Condition{
						Type:   gardencorev1beta1.ShootHibernationPossible,
						Status: gardencorev1beta1.ConditionFalse,
					}

					shoot.Status = gardencorev1beta1.ShootStatus{
						Conditions:  []gardencorev1beta1.Condition{apiServerCondition},
						Constraints: []gardencorev1beta1.Condition{hibernationConstraint},
					}
					Expect(gardenClient.Update(ctx, shoot)).To(Succeed())

					Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedShoot := &gardencorev1beta1.Shoot{}
					Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), updatedShoot)).To(Succeed())
					Expect(updatedShoot.Status.Conditions).To(ConsistOf(apiServerCondition))
					Expect(updatedShoot.Status.Constraints).To(ConsistOf(hibernationConstraint))
				})
			})

			Context("when conditions / constraints are changed", func() {
				var conditions, constraints []gardencorev1beta1.Condition

				BeforeEach(func() {
					conditions = []gardencorev1beta1.Condition{
						{
							Type:   gardencorev1beta1.ShootAPIServerAvailable,
							Status: gardencorev1beta1.ConditionTrue,
							Reason: "foo",
						},
						{
							Type:   gardencorev1beta1.ShootControlPlaneHealthy,
							Status: gardencorev1beta1.ConditionFalse,
							Reason: "bar",
						},
						{
							Type:   gardencorev1beta1.ShootObservabilityComponentsHealthy,
							Status: gardencorev1beta1.ConditionFalse,
							Reason: "dash",
						},
						{
							Type:   gardencorev1beta1.ShootEveryNodeReady,
							Status: gardencorev1beta1.ConditionProgressing,
						},
						{
							Type:    gardencorev1beta1.ShootSystemComponentsHealthy,
							Status:  gardencorev1beta1.ConditionFalse,
							Message: "foo bar",
						},
					}

					constraints = []gardencorev1beta1.Condition{
						{
							Type:   gardencorev1beta1.ShootHibernationPossible,
							Status: gardencorev1beta1.ConditionProgressing,
							Reason: "foo",
						},
						{
							Type:   gardencorev1beta1.ShootMaintenancePreconditionsSatisfied,
							Status: gardencorev1beta1.ConditionFalse,
							Reason: "bar",
						},
					}

					DeferCleanup(test.WithVars(
						&NewHealthCheck, healthCheckFunc(func(cond []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
							return conditions
						}),
						&NewConstraintCheck, constraintCheckFunc(func(constr []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
							return constraints
						}),
					))
				})

				It("should update shoot conditions", func() {
					Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

					updatedShoot := &gardencorev1beta1.Shoot{}
					Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), updatedShoot)).To(Succeed())
					Expect(updatedShoot.Status.Conditions).To(ConsistOf(conditions))
					Expect(updatedShoot.Status.Constraints).To(ConsistOf(constraints))
				})

				Context("when shoot doesn't have a last operation", func() {
					It("should update the shoot conditions", func() {
						apiServerCondition := gardencorev1beta1.Condition{
							Type:   gardencorev1beta1.ShootAPIServerAvailable,
							Status: gardencorev1beta1.ConditionUnknown,
						}

						hibernationConstraint := gardencorev1beta1.Condition{
							Type:   gardencorev1beta1.ShootHibernationPossible,
							Status: gardencorev1beta1.ConditionFalse,
						}

						shoot.Status = gardencorev1beta1.ShootStatus{
							Conditions:  []gardencorev1beta1.Condition{apiServerCondition},
							Constraints: []gardencorev1beta1.Condition{hibernationConstraint},
						}
						Expect(gardenClient.Update(ctx, shoot)).To(Succeed())

						Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

						updatedShoot := &gardencorev1beta1.Shoot{}
						Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), updatedShoot)).To(Succeed())
						Expect(updatedShoot.Status.Conditions).To(ConsistOf(conditions))
						Expect(updatedShoot.Status.Constraints).To(ConsistOf(constraints))
					})
				})

				Context("when shoot has a successful last operation", func() {
					BeforeEach(func() {
						shoot.Status = gardencorev1beta1.ShootStatus{
							LastOperation: &gardencorev1beta1.LastOperation{
								Type:  gardencorev1beta1.LastOperationTypeReconcile,
								State: gardencorev1beta1.LastOperationStateSucceeded,
							},
						}
					})

					It("should set shoot to unhealthy", func() {
						Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

						updatedShoot := &gardencorev1beta1.Shoot{}
						Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), updatedShoot)).To(Succeed())
						Expect(updatedShoot.Status.Conditions).To(ConsistOf(conditions))
						Expect(updatedShoot.Status.Constraints).To(ConsistOf(constraints))
					})
				})
			})

			Context("when conditions / constraints are changed to healthy", func() {
				var conditions, constraints []gardencorev1beta1.Condition

				BeforeEach(func() {
					conditions = []gardencorev1beta1.Condition{
						{
							Type:   gardencorev1beta1.ShootAPIServerAvailable,
							Status: gardencorev1beta1.ConditionTrue,
							Reason: "foo",
						},
						{
							Type:   gardencorev1beta1.ShootControlPlaneHealthy,
							Status: gardencorev1beta1.ConditionTrue,
							Reason: "bar",
						},
						{
							Type:           gardencorev1beta1.ShootEveryNodeReady,
							Status:         gardencorev1beta1.ConditionTrue,
							LastUpdateTime: metav1.NewTime(metav1.Now().Round(time.Second)),
						},
						{
							Type:    gardencorev1beta1.ShootSystemComponentsHealthy,
							Status:  gardencorev1beta1.ConditionTrue,
							Message: "foo bar",
						},
					}

					constraints = []gardencorev1beta1.Condition{
						{
							Type:   gardencorev1beta1.ShootHibernationPossible,
							Status: gardencorev1beta1.ConditionTrue,
							Reason: "foo",
						},
						{
							Type:   gardencorev1beta1.ShootMaintenancePreconditionsSatisfied,
							Status: gardencorev1beta1.ConditionTrue,
							Reason: "bar",
						},
					}

					DeferCleanup(test.WithVars(
						&NewHealthCheck, healthCheckFunc(func(cond []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
							return conditions
						}),
						&NewConstraintCheck, constraintCheckFunc(func(constr []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
							return constraints
						}),
					))
				})

				Context("when shoot has a successful last operation", func() {
					BeforeEach(func() {
						shoot.Status = gardencorev1beta1.ShootStatus{
							LastOperation: &gardencorev1beta1.LastOperation{
								Type:  gardencorev1beta1.LastOperationTypeReconcile,
								State: gardencorev1beta1.LastOperationStateSucceeded,
							},
						}
					})

					It("should set shoot to healthy", func() {
						Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{RequeueAfter: careSyncPeriod}))

						updatedShoot := &gardencorev1beta1.Shoot{}
						Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), updatedShoot)).To(Succeed())
						Expect(updatedShoot.Status.Conditions).To(ConsistOf(conditions))
					})
				})
			})
		})
	})
})

type resultingConditionFunc func(cond []gardencorev1beta1.Condition) []gardencorev1beta1.Condition

func (h resultingConditionFunc) Check(_ context.Context, _ map[gardencorev1beta1.ConditionType]time.Duration, _ *metav1.Duration, con []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
	return h(con)
}

func healthCheckFunc(fn resultingConditionFunc) NewHealthCheckFunc {
	return func(op *operation.Operation, init care.ShootClientInit, clock clock.Clock) HealthCheck {
		return fn
	}
}

type resultingConstraintFunc func(cond []gardencorev1beta1.Condition) []gardencorev1beta1.Condition

func (c resultingConstraintFunc) Check(_ context.Context, constraints []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
	return c(constraints)
}

func constraintCheckFunc(fn resultingConstraintFunc) NewConstraintCheckFunc {
	return func(clock clock.Clock, op *operation.Operation, init care.ShootClientInit) ConstraintCheck {
		return fn
	}
}

func opFunc(op *operation.Operation, err error) NewOperationFunc {
	return func(
		_ context.Context,
		_ logr.Logger,
		_ client.Client,
		_ kubernetes.Interface,
		_ clientmap.ClientMap,
		_ *config.GardenletConfiguration,
		_ *gardencorev1beta1.Gardener,
		_ string,
		_ map[string]*corev1.Secret,
		_ imagevector.ImageVector,
		_ *gardencorev1beta1.Shoot,
	) (*operation.Operation, error) {
		return op, err
	}
}

type nopGarbageCollector struct{}

func (n *nopGarbageCollector) Collect(_ context.Context) {}

func nopGarbageCollectorFunc() NewGarbageCollectorFunc {
	return func(_ *operation.Operation, _ care.ShootClientInit) GarbageCollector {
		return &nopGarbageCollector{}
	}
}

func consistOfConditionsInUnknownStatus(message string, isWorkerless bool) types.GomegaMatcher {
	var len = 3
	matcher := And(
		ContainCondition(
			OfType(gardencorev1beta1.ShootAPIServerAvailable),
			WithStatus(gardencorev1beta1.ConditionUnknown),
			WithMessage(message),
		),
		ContainCondition(
			OfType(gardencorev1beta1.ShootControlPlaneHealthy),
			WithStatus(gardencorev1beta1.ConditionUnknown),
			WithMessage(message),
		),
		ContainCondition(
			OfType(gardencorev1beta1.ShootObservabilityComponentsHealthy),
			WithStatus(gardencorev1beta1.ConditionUnknown),
			WithMessage(message),
		),
	)

	if !isWorkerless {
		len = 5
		matcher = And(matcher,
			ContainCondition(
				OfType(gardencorev1beta1.ShootEveryNodeReady),
				WithStatus(gardencorev1beta1.ConditionUnknown),
				WithMessage(message),
			),
			ContainCondition(
				OfType(gardencorev1beta1.ShootSystemComponentsHealthy),
				WithStatus(gardencorev1beta1.ConditionUnknown),
				WithMessage(message),
			),
		)
	}

	return And(matcher, HaveLen(len))
}

func consistOfConstraintsInUnknownStatus(message string) types.GomegaMatcher {
	return ConsistOf(
		MatchFields(IgnoreExtras, Fields{
			"Type":    Equal(gardencorev1beta1.ShootHibernationPossible),
			"Status":  Equal(gardencorev1beta1.ConditionUnknown),
			"Message": Equal(message),
		}),
		MatchFields(IgnoreExtras, Fields{
			"Type":    Equal(gardencorev1beta1.ShootMaintenancePreconditionsSatisfied),
			"Status":  Equal(gardencorev1beta1.ConditionUnknown),
			"Message": Equal(message),
		}),
		MatchFields(IgnoreExtras, Fields{
			"Type":    Equal(gardencorev1beta1.ShootCACertificateValiditiesAcceptable),
			"Status":  Equal(gardencorev1beta1.ConditionUnknown),
			"Message": Equal(message),
		}),
	)
}
