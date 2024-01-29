// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package framework_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/test/framework"
)

var _ = Describe("Kubernetes Utils", func() {
	Describe("#ShootReconciliationSuccessful", func() {
		var (
			shoot *gardencorev1beta1.Shoot

			testShootReconcilationSuccessful = func(matchMessage, matchResult types.GomegaMatcher) {
				successful, msg := framework.ShootReconciliationSuccessful(shoot)
				Expect(msg).To(matchMessage)
				Expect(successful).To(matchResult)
			}
		)

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 1,
				},
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{
							{Name: "worker"},
						},
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					ObservedGeneration: 1,
				},
			}
		})

		Context("when lastOperation is Succeeded and all conditions are True", func() {
			BeforeEach(func() {
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeCreate,
					State: gardencorev1beta1.LastOperationStateSucceeded,
				}
			})

			It("should return true if shoot is reconciled successfully", func() {
				appendShootConditionsToShoot(shoot)

				testShootReconcilationSuccessful(BeEmpty(), BeTrue())
			})

			It("should return true if workerless shoot is reconciled successfully", func() {
				shoot.Spec.Provider.Workers = nil
				appendShootConditionsToShoot(shoot)

				testShootReconcilationSuccessful(BeEmpty(), BeTrue())
			})

			It("should return true if shoot which acts as seed is reconciled successfully", func() {
				appendShootConditionsToShoot(shoot)
				appendSeedConditionsToShoot(shoot)

				testShootReconcilationSuccessful(BeEmpty(), BeTrue())
			})
		})

		Context("when generation is outdated", func() {
			It("should return false and appropriate message", func() {
				shoot.Status.ObservedGeneration = 0

				testShootReconcilationSuccessful(ContainSubstring("generation did not equal observed generation"), BeFalse())
			})
		})

		Context("when lastOperation and conditions are not set", func() {
			It("should return false and appropriate message", func() {
				shoot.Status.ObservedGeneration = 1

				testShootReconcilationSuccessful(ContainSubstring("no conditions and last operation present yet"), BeFalse())
			})
		})

		Context("when not all conditions are True", func() {
			It("should return false and appropriate message if not all conditions are True", func() {
				appendShootConditionsToShoot(shoot)
				setConditionToFalse(shoot, gardencorev1beta1.ShootSystemComponentsHealthy)

				testShootReconcilationSuccessful(ContainSubstring("condition type SystemComponentsHealthy is not true yet"), BeFalse())
			})

			It("should return false and appropriate message if not all conditions are True for workerless shoot", func() {
				shoot.Spec.Provider.Workers = nil
				appendShootConditionsToShoot(shoot)
				setConditionToFalse(shoot, gardencorev1beta1.ShootControlPlaneHealthy)

				testShootReconcilationSuccessful(ContainSubstring("condition type ControlPlaneHealthy is not true yet"), BeFalse())
			})

			It("should return false and appropriate message if shoot acts as seed and a seed condition is not True", func() {
				appendShootConditionsToShoot(shoot)
				appendSeedConditionsToShoot(shoot)
				setConditionToFalse(shoot, gardencorev1beta1.SeedExtensionsReady)

				testShootReconcilationSuccessful(ContainSubstring("condition type ExtensionsReady is not true yet"), BeFalse())
			})

			It("should return false and appropriate message if shoot acts as seed, not all shoot conditions are true and shoot is being hibernated", func() {
				shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
					Enabled: ptr.To(true),
				}

				appendShootConditionsToShoot(shoot)
				appendSeedConditionsToShoot(shoot)
				setConditionToFalse(shoot, gardencorev1beta1.ShootSystemComponentsHealthy)

				testShootReconcilationSuccessful(ContainSubstring("condition type SystemComponentsHealthy is not true yet"), BeFalse())
			})

			It("should return true and empty message if shoot acts as seed, not all seed conditions are true and shoot is being hibernated", func() {
				shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
					Enabled: ptr.To(true),
				}

				appendShootConditionsToShoot(shoot)
				appendSeedConditionsToShoot(shoot)
				setConditionToFalse(shoot, gardencorev1beta1.SeedExtensionsReady)

				testShootReconcilationSuccessful(BeEmpty(), BeTrue())
			})
		})

		Context("when lastOperation is not Succeeded", func() {
			BeforeEach(func() {
				appendShootConditionsToShoot(shoot)
			})

			DescribeTable("when lastOperation is",
				func(lastOperation *gardencorev1beta1.LastOperation, matchMessage, matchResult types.GomegaMatcher) {
					shoot.Status.LastOperation = lastOperation
					testShootReconcilationSuccessful(matchMessage, matchResult)
				},
				Entry("Create",
					&gardencorev1beta1.LastOperation{
						Type:  gardencorev1beta1.LastOperationTypeCreate,
						State: gardencorev1beta1.LastOperationStateFailed,
					},
					ContainSubstring("last operation type was create, reconcile or restore but state was not succeeded"),
					BeFalse(),
				),
				Entry("Reconcile",
					&gardencorev1beta1.LastOperation{
						Type:  gardencorev1beta1.LastOperationTypeReconcile,
						State: gardencorev1beta1.LastOperationStateFailed,
					},
					ContainSubstring("last operation type was create, reconcile or restore but state was not succeeded"),
					BeFalse(),
				),
				Entry("Migrate Failed",
					&gardencorev1beta1.LastOperation{
						Type:  gardencorev1beta1.LastOperationTypeMigrate,
						State: gardencorev1beta1.LastOperationStateFailed,
					},
					ContainSubstring("last operation type was migrate, the migration process is not finished yet"),
					BeFalse(),
				),
				Entry("Mgrate Succeeded",
					&gardencorev1beta1.LastOperation{
						Type:  gardencorev1beta1.LastOperationTypeMigrate,
						State: gardencorev1beta1.LastOperationStateSucceeded,
					},
					ContainSubstring("last operation type was migrate, the migration process is not finished yet"),
					BeFalse(),
				),
				Entry("Restore",
					&gardencorev1beta1.LastOperation{
						Type:  gardencorev1beta1.LastOperationTypeMigrate,
						State: gardencorev1beta1.LastOperationStateFailed,
					},
					ContainSubstring("last operation type was migrate, the migration process is not finished yet"),
					BeFalse(),
				),
			)
		})
	})
})

func appendShootConditionsToShoot(shoot *gardencorev1beta1.Shoot) {
	shoot.Status.Conditions = append(shoot.Status.Conditions, []gardencorev1beta1.Condition{
		{
			Type:   gardencorev1beta1.ShootAPIServerAvailable,
			Status: gardencorev1beta1.ConditionTrue,
		},
		{
			Type:   gardencorev1beta1.ShootControlPlaneHealthy,
			Status: gardencorev1beta1.ConditionTrue,
		},
		{
			Type:   gardencorev1beta1.ShootObservabilityComponentsHealthy,
			Status: gardencorev1beta1.ConditionTrue,
		},
		{
			Type:   gardencorev1beta1.ShootSystemComponentsHealthy,
			Status: gardencorev1beta1.ConditionTrue,
		},
	}...,
	)

	if !v1beta1helper.IsWorkerless(shoot) {
		shoot.Status.Conditions = append(shoot.Status.Conditions, []gardencorev1beta1.Condition{
			{
				Type:   gardencorev1beta1.ShootEveryNodeReady,
				Status: gardencorev1beta1.ConditionTrue,
			},
		}...,
		)
	}
}

func appendSeedConditionsToShoot(shoot *gardencorev1beta1.Shoot) {
	shoot.Status.Conditions = append(shoot.Status.Conditions, []gardencorev1beta1.Condition{
		{
			Type:   gardencorev1beta1.SeedGardenletReady,
			Status: gardencorev1beta1.ConditionTrue,
		},
		{
			Type:   gardencorev1beta1.SeedBackupBucketsReady,
			Status: gardencorev1beta1.ConditionTrue,
		},
		{
			Type:   gardencorev1beta1.SeedSystemComponentsHealthy,
			Status: gardencorev1beta1.ConditionTrue,
		},
		{
			Type:   gardencorev1beta1.SeedExtensionsReady,
			Status: gardencorev1beta1.ConditionTrue,
		},
	}...)
}

func setConditionToFalse(shoot *gardencorev1beta1.Shoot, conditionType gardencorev1beta1.ConditionType) {
	for i, condition := range shoot.Status.Conditions {
		if condition.Type == conditionType {
			shoot.Status.Conditions[i].Status = gardencorev1beta1.ConditionFalse
			return
		}
	}
}
