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

package shoot_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/scheduler/controller/shoot"
)

var _ = Describe("Add", func() {
	var reconciler *Reconciler

	BeforeEach(func() {
		reconciler = &Reconciler{}
	})

	Describe("ShootPredicate", func() {
		var (
			predicate predicate.Predicate
			shoot     *gardencorev1beta1.Shoot

			createEvent  event.CreateEvent
			updateEvent  event.UpdateEvent
			deleteEvent  event.DeleteEvent
			genericEvent event.GenericEvent
		)

		BeforeEach(func() {
			predicate = reconciler.ShootPredicate()
			shoot = &gardencorev1beta1.Shoot{}

			createEvent = event.CreateEvent{
				Object: shoot,
			}
			updateEvent = event.UpdateEvent{
				ObjectOld: shoot,
				ObjectNew: shoot,
			}
			deleteEvent = event.DeleteEvent{
				Object: shoot,
			}
			genericEvent = event.GenericEvent{
				Object: shoot,
			}
		})

		Context("shoot is unassigned", func() {
			It("should be true", func() {
				Expect(predicate.Create(createEvent)).To(BeTrue())
				Expect(predicate.Update(updateEvent)).To(BeTrue())
				Expect(predicate.Delete(deleteEvent)).To(BeTrue())
				Expect(predicate.Generic(genericEvent)).To(BeTrue())
			})
		})

		Context("shoot is assigned", func() {
			BeforeEach(func() {
				shoot.Spec.SeedName = pointer.String("seed")
			})

			It("should be false", func() {
				Expect(predicate.Create(createEvent)).To(BeFalse())
				Expect(predicate.Update(updateEvent)).To(BeFalse())
				Expect(predicate.Delete(deleteEvent)).To(BeFalse())
				Expect(predicate.Generic(genericEvent)).To(BeFalse())
			})
		})
	})
})
