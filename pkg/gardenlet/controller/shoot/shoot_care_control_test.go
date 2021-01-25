// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot_test

import (
	"context"
	"errors"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/core/clientset/versioned/fake"
	"github.com/gardener/gardener/pkg/client/core/informers/externalversions/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/care"
	"github.com/gardener/gardener/pkg/operation/common"
	operationshoot "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Shoot Care Control", func() {
	var (
		careControl   CareControlInterface
		coreInformers v1beta1.Interface
		gardenletConf *config.GardenletConfiguration

		operationFunc NewOperationFunc
	)

	BeforeSuite(func() {
		logger.Logger = logger.NewNopLogger()
	})

	AfterEach(func() {
		careControl = nil
	})

	Describe("#Care", func() {
		var (
			seedName, shootName, shootNamespace, key string
			shoot                                    *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			seedName = "seed"
			shootName = "shoot"
			shootNamespace = "project"
			key = kutil.Key(shootNamespace, shootName).String()

			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: shootNamespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: &seedName,
				},
			}

			gardenletConf = &config.GardenletConfiguration{
				Controllers: &config.GardenletControllerConfiguration{
					ShootCare: &config.ShootCareControllerConfiguration{
						SyncPeriod: &metav1.Duration{Duration: 60 * time.Second},
					},
				},
			}
		})

		Context("when health check setup is broken", func() {
			var (
				gardenCoreClient *fake.Clientset
				clientMapBuilder *fakeclientmap.ClientMapBuilder
			)

			JustBeforeEach(func() {
				gardenCoreClient = fake.NewSimpleClientset(shoot)
				gardenClientSet := fakeclientset.NewClientSetBuilder().
					WithGardenCore(gardenCoreClient).
					Build()
				clientMapBuilder.WithClientSetForKey(keys.ForGarden(), gardenClientSet)
			})

			BeforeEach(func() {
				clientMapBuilder = fakeclientmap.NewClientMapBuilder()
			})

			Context("when operation cannot be created", func() {
				BeforeEach(func() {
					clientMapBuilder.WithClientSetForKey(keys.ForSeedWithName(seedName), fakeclientset.NewClientSet())
				})
				It("should report a setup failure", func() {
					NewOperation = opFunc(nil, errors.New("foo"))
					careControl = NewDefaultCareControl(clientMapBuilder.Build(), coreInformers, nil, nil, nil, "", gardenletConf)

					Expect(careControl.Care(shoot, key)).To(Succeed())
					updatedShoot, err := gardenCoreClient.CoreV1beta1().Shoots(shootNamespace).Get(context.Background(), shootName, metav1.GetOptions{})
					Expect(err).To(Not(HaveOccurred()))
					Expect(updatedShoot.Status.Conditions).To(consistOfConditionsInUnknownStatus("Precondition failed: operation could not be initialized"))
					Expect(updatedShoot.Status.Constraints).To(consistOfConstraintsInUnknownStatus("Precondition failed: operation could not be initialized"))
				})
			})

			Context("when seed client is not available", func() {
				BeforeEach(func() {
					shoot = &gardencorev1beta1.Shoot{
						ObjectMeta: metav1.ObjectMeta{
							Name:      shootName,
							Namespace: shootNamespace,
						},
						Spec: gardencorev1beta1.ShootSpec{
							SeedName: &shootName,
						},
					}
				})

				It("should report a setup failure", func() {
					careControl = NewDefaultCareControl(clientMapBuilder.Build(), coreInformers, nil, nil, nil, "", gardenletConf)
					Expect(careControl.Care(shoot, key)).To(Succeed())
					updatedShoot, err := gardenCoreClient.CoreV1beta1().Shoots(shootNamespace).Get(context.Background(), shootName, metav1.GetOptions{})
					Expect(err).To(Not(HaveOccurred()))
					Expect(updatedShoot.Status.Conditions).To(consistOfConditionsInUnknownStatus("Precondition failed: seed client cannot be constructed"))
					Expect(updatedShoot.Status.Constraints).To(consistOfConstraintsInUnknownStatus("Precondition failed: seed client cannot be constructed"))
				})
			})
		})

		Context("when health check setup is successful", func() {
			var (
				ctrl                     *gomock.Controller
				seedClient, gardenClient *mockclient.MockClient
				clientMap                clientmap.ClientMap
				gardenCoreClient         *fake.Clientset

				shootedSeed *gardencorev1beta1helper.ShootedSeed
			)

			JustBeforeEach(func() {
				gardenCoreClient = fake.NewSimpleClientset(shoot)

				gardenClientSet := fakeclientset.NewClientSetBuilder().
					WithGardenCore(gardenCoreClient).
					WithClient(gardenClient).
					Build()
				seedClientSet := fakeclientset.NewClientSetBuilder().
					WithClient(seedClient).
					Build()
				clientMap = fakeclientmap.NewClientMapBuilder().
					WithClientSetForKey(keys.ForGarden(), gardenClientSet).
					WithClientSetForKey(keys.ForSeedWithName(seedName), seedClientSet).
					Build()

				operationFunc = opFunc(&operation.Operation{
					K8sGardenClient: gardenClientSet,
					K8sSeedClient:   seedClientSet,
					ShootedSeed:     shootedSeed,
					Shoot: &operationshoot.Shoot{
						Info: shoot,
					},
					Logger: logger.NewNopLogger().WithContext(context.Background()),
				}, nil)

				NewOperation = operationFunc
				NewGarbageCollector = nopGarbageCollectorFunc()
				careControl = NewDefaultCareControl(clientMap, coreInformers, nil, nil, nil, "", gardenletConf)
			})

			BeforeEach(func() {
				ctrl = gomock.NewController(GinkgoT())

				seedClient = mockclient.NewMockClient(ctrl)
				gardenClient = mockclient.NewMockClient(ctrl)
			})

			AfterEach(func() {
				ctrl.Finish()
				shoot = nil
			})

			Context("when no conditions / constraints are returned", func() {
				BeforeEach(func() {
					NewHealthCheck = healthCheckFunc(func(_ []gardencorev1beta1.Condition) []gardencorev1beta1.Condition { return nil })
					NewConstraintCheck = constraintCheckFunc(func(_ []gardencorev1beta1.Condition) []gardencorev1beta1.Condition { return nil })
				})
				It("should not set conditions / constraints", func() {
					var updatedShoot *gardencorev1beta1.Shoot
					gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
						func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch) error {
							updatedShoot = shoot
							return nil
						})
					Expect(careControl.Care(shoot, key)).To(Succeed())
					Expect(updatedShoot.Status.Conditions).To(BeEmpty())
					Expect(updatedShoot.Status.Constraints).To(BeEmpty())
					Expect(updatedShoot.ObjectMeta.Labels).Should(HaveKeyWithValue(common.ShootStatus, string(operationshoot.StatusHealthy)))
				})
				It("should remove conditions / constraints", func() {
					apiServerCondition := gardencorev1beta1.Condition{
						Type:           gardencorev1beta1.ShootAPIServerAvailable,
						Status:         gardencorev1beta1.ConditionTrue,
						LastUpdateTime: metav1.Now(),
					}

					hibernationConstraint := gardencorev1beta1.Condition{
						Type:           gardencorev1beta1.ShootHibernationPossible,
						Status:         gardencorev1beta1.ConditionFalse,
						LastUpdateTime: metav1.Now(),
					}

					shoot.Status = gardencorev1beta1.ShootStatus{
						Conditions:  []gardencorev1beta1.Condition{apiServerCondition},
						Constraints: []gardencorev1beta1.Condition{hibernationConstraint},
					}

					var updatedShoot *gardencorev1beta1.Shoot
					gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
						func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch) error {
							updatedShoot = shoot
							return nil
						})
					Expect(careControl.Care(shoot, key)).To(Succeed())
					Expect(updatedShoot.Status.Conditions).To(BeEmpty())
					Expect(updatedShoot.Status.Constraints).To(BeEmpty())
					Expect(updatedShoot.ObjectMeta.Labels).Should(HaveKeyWithValue(common.ShootStatus, string(operationshoot.StatusHealthy)))
				})
			})

			Context("when conditions / constraints are returned unchanged", func() {
				BeforeEach(func() {
					NewHealthCheck = healthCheckFunc(func(cond []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
						copy := append(cond[:0:0], cond...)
						return copy
					})
					NewConstraintCheck = constraintCheckFunc(func(constr []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
						copy := append(constr[:0:0], constr...)
						return copy
					})
				})
				It("should not set conditions / constraints", func() {
					var updatedShoot *gardencorev1beta1.Shoot
					gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
						func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch) error {
							updatedShoot = shoot
							return nil
						})
					Expect(careControl.Care(shoot, key)).To(Succeed())
					Expect(updatedShoot.Status.Conditions).To(BeEmpty())
					Expect(updatedShoot.ObjectMeta.Labels).Should(HaveKeyWithValue(common.ShootStatus, string(operationshoot.StatusHealthy)))
				})
				It("should not amend existing conditions / constraints", func() {
					apiServerCondition := gardencorev1beta1.Condition{
						Type:           gardencorev1beta1.ShootAPIServerAvailable,
						Status:         gardencorev1beta1.ConditionTrue,
						LastUpdateTime: metav1.Now(),
					}

					hibernationConstraint := gardencorev1beta1.Condition{
						Type:           gardencorev1beta1.ShootHibernationPossible,
						Status:         gardencorev1beta1.ConditionFalse,
						LastUpdateTime: metav1.Now(),
					}

					shoot.Status = gardencorev1beta1.ShootStatus{
						Conditions:  []gardencorev1beta1.Condition{apiServerCondition},
						Constraints: []gardencorev1beta1.Condition{hibernationConstraint},
					}

					var updatedShoot *gardencorev1beta1.Shoot
					gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
						func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch) error {
							updatedShoot = shoot
							return nil
						})
					Expect(careControl.Care(shoot, key)).To(Succeed())
					Expect(updatedShoot.Status.Conditions).To(ConsistOf(apiServerCondition))
					Expect(updatedShoot.Status.Constraints).To(ConsistOf(hibernationConstraint))
					Expect(updatedShoot.ObjectMeta.Labels).Should(HaveKeyWithValue(common.ShootStatus, string(operationshoot.StatusHealthy)))
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
							Type:           gardencorev1beta1.ShootEveryNodeReady,
							Status:         gardencorev1beta1.ConditionProgressing,
							LastUpdateTime: metav1.Now(),
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

					NewHealthCheck = healthCheckFunc(func(cond []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
						return conditions
					})
					NewConstraintCheck = constraintCheckFunc(func(constr []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
						return constraints
					})
				})

				Context("when shoot is a seed", func() {
					var (
						seed           *gardencorev1beta1.Seed
						seedConditions []gardencorev1beta1.Condition
					)

					BeforeEach(func() {
						seedConditions = []gardencorev1beta1.Condition{
							{
								Type:    gardencorev1beta1.SeedBootstrapped,
								Status:  gardencorev1beta1.ConditionTrue,
								Message: "foo",
							},
							{
								Type:   gardencorev1beta1.SeedExtensionsReady,
								Status: gardencorev1beta1.ConditionProgressing,
								Reason: "bar",
							},
						}

						seed = &gardencorev1beta1.Seed{
							ObjectMeta: metav1.ObjectMeta{
								Name: shootName,
							},
							Status: gardencorev1beta1.SeedStatus{
								Conditions: seedConditions,
							},
						}

						shootedSeed = &gardencorev1beta1helper.ShootedSeed{}

						gardenClient.EXPECT().Get(gomock.Any(), kutil.Key(seed.Name), gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).DoAndReturn(
							func(_ context.Context, _ client.ObjectKey, s *gardencorev1beta1.Seed) error {
								*s = *seed
								return nil
							})
					})

					AfterEach(func() {
						shootedSeed = nil
					})

					It("should merge shoot and seed conditions", func() {
						var updatedShoot *gardencorev1beta1.Shoot
						gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
							func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch) error {
								updatedShoot = shoot
								return nil
							})
						Expect(careControl.Care(shoot, key)).To(Succeed())
						Expect(updatedShoot.Status.Conditions).To(ConsistOf(append(conditions, seedConditions...)))
						Expect(updatedShoot.Status.Constraints).To(ConsistOf(constraints))
						Expect(updatedShoot.ObjectMeta.Labels).Should(HaveKeyWithValue(common.ShootStatus, string(operationshoot.StatusHealthy)))
					})
				})

				Context("when shoot doesn't have a last operation", func() {
					It("should update the shoot conditions", func() {
						apiServerCondition := gardencorev1beta1.Condition{
							Type:           gardencorev1beta1.ShootAPIServerAvailable,
							Status:         gardencorev1beta1.ConditionUnknown,
							LastUpdateTime: metav1.Now(),
						}

						hibernationConstraint := gardencorev1beta1.Condition{
							Type:           gardencorev1beta1.ShootHibernationPossible,
							Status:         gardencorev1beta1.ConditionFalse,
							LastUpdateTime: metav1.Now(),
						}

						shoot.Status = gardencorev1beta1.ShootStatus{
							Conditions:  []gardencorev1beta1.Condition{apiServerCondition},
							Constraints: []gardencorev1beta1.Condition{hibernationConstraint},
						}

						var updatedShoot *gardencorev1beta1.Shoot
						gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
							func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch) error {
								updatedShoot = shoot
								return nil
							})
						Expect(careControl.Care(shoot, key)).To(Succeed())
						Expect(updatedShoot.Status.Conditions).To(ConsistOf(conditions))
						Expect(updatedShoot.Status.Constraints).To(ConsistOf(constraints))
						Expect(updatedShoot.ObjectMeta.Labels).Should(HaveKeyWithValue(common.ShootStatus, string(operationshoot.StatusHealthy)))
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
						var updatedShoot *gardencorev1beta1.Shoot
						gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
							func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch) error {
								updatedShoot = shoot
								return nil
							})
						Expect(careControl.Care(shoot, key)).To(Succeed())
						Expect(updatedShoot.Status.Conditions).To(ConsistOf(conditions))
						Expect(updatedShoot.Status.Constraints).To(ConsistOf(constraints))
						Expect(updatedShoot.ObjectMeta.Labels).Should(HaveKeyWithValue(common.ShootStatus, string(operationshoot.StatusUnhealthy)))
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
	return func(op *operation.Operation, init care.ShootClientInit) HealthCheck {
		return fn
	}
}

type resultingConstraintFunc func(cond []gardencorev1beta1.Condition) []gardencorev1beta1.Condition

func (c resultingConstraintFunc) Check(ctx context.Context, constraints []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
	return c(constraints)
}

func constraintCheckFunc(fn resultingConstraintFunc) NewConstraintCheckFunc {
	return func(op *operation.Operation, init care.ShootClientInit) ConstraintCheck {
		return fn
	}

}

func opFunc(op *operation.Operation, err error) NewOperationFunc {
	return func(
		_ context.Context,
		_ kubernetes.Interface,
		_ *config.GardenletConfiguration,
		_ *gardencorev1beta1.Gardener,
		_ string,
		_ map[string]*corev1.Secret,
		_ imagevector.ImageVector,
		_ v1beta1.Interface,
		_ clientmap.ClientMap,
		_ *gardencorev1beta1.Shoot,
		_ *logrus.Entry,
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

func consistOfConditionsInUnknownStatus(message string) types.GomegaMatcher {
	return ConsistOf(
		MatchFields(IgnoreExtras, Fields{
			"Type":    Equal(gardencorev1beta1.ShootAPIServerAvailable),
			"Status":  Equal(gardencorev1beta1.ConditionUnknown),
			"Message": Equal(message),
		}),
		MatchFields(IgnoreExtras, Fields{
			"Type":    Equal(gardencorev1beta1.ShootControlPlaneHealthy),
			"Status":  Equal(gardencorev1beta1.ConditionUnknown),
			"Message": Equal(message),
		}),
		MatchFields(IgnoreExtras, Fields{
			"Type":    Equal(gardencorev1beta1.ShootEveryNodeReady),
			"Status":  Equal(gardencorev1beta1.ConditionUnknown),
			"Message": Equal(message),
		}),
		MatchFields(IgnoreExtras, Fields{
			"Type":    Equal(gardencorev1beta1.ShootSystemComponentsHealthy),
			"Status":  Equal(gardencorev1beta1.ConditionUnknown),
			"Message": Equal(message),
		}),
	)
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
	)
}
