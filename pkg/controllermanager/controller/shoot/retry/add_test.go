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

package retry_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/shoot/retry"
)

var _ = Describe("Add", func() {
	var reconciler *Reconciler

	BeforeEach(func() {
		reconciler = &Reconciler{}
	})

	Describe("ShootPredicate", func() {
		var (
			p     predicate.Predicate
			shoot *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			p = reconciler.ShootPredicate()
			shoot = &gardencorev1beta1.Shoot{}
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because new object is no shoot", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is no shoot", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot})).To(BeFalse())
			})

			It("should return false because shoot did not fail", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Status = gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateProcessing,
					},
				}
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeFalse())
			})

			It("should return false because old shoot was already failed", func() {
				shoot.Status = gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateFailed,
					},
				}
				oldShoot := shoot.DeepCopy()
				shoot.Status.LastErrors = []gardencorev1beta1.LastError{{
					Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorConfigurationProblem},
				}}
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeFalse())
			})

			It("should return false because shoot did not fail due to rate limits", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Status = gardencorev1beta1.ShootStatus{
					LastErrors: []gardencorev1beta1.LastError{{
						Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorConfigurationProblem},
					}},
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateFailed,
					},
				}
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeFalse())
			})

			It("should return true because shoot failed due to rate limits", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Status = gardencorev1beta1.ShootStatus{
					LastErrors: []gardencorev1beta1.LastError{{
						Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraRateLimitsExceeded},
					}},
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateFailed,
					},
				}
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return false", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeFalse())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})
})
