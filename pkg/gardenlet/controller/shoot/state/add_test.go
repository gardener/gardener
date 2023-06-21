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

package state_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/state"
)

var _ = Describe("Add", func() {
	var (
		reconciler *Reconciler
		shoot      *gardencorev1beta1.Shoot

		seedName = "seed"
	)

	BeforeEach(func() {
		reconciler = &Reconciler{SeedName: seedName}
		shoot = &gardencorev1beta1.Shoot{}
	})

	Describe("#SeedNamePredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.SeedNamePredicate()
		})

		It("should return false because new object is no shoot", func() {
			Expect(p.Create(event.CreateEvent{})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{})).To(BeFalse())
		})

		It("should return false because seed name is not set", func() {
			Expect(p.Create(event.CreateEvent{Object: shoot})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: shoot})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: shoot})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: shoot})).To(BeFalse())
		})

		It("should return false because seed name does not match", func() {
			shoot.Spec.SeedName = pointer.String("some-seed")

			Expect(p.Create(event.CreateEvent{Object: shoot})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: shoot})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: shoot})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: shoot})).To(BeFalse())
		})

		It("should return true because seed name matches", func() {
			shoot.Spec.SeedName = &seedName

			Expect(p.Create(event.CreateEvent{Object: shoot})).To(BeTrue())
			Expect(p.Update(event.UpdateEvent{ObjectNew: shoot})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: shoot})).To(BeTrue())
			Expect(p.Generic(event.GenericEvent{Object: shoot})).To(BeTrue())
		})
	})

	Describe("#SeedNameChangedPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.SeedNameChangedPredicate()
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

			It("should return false because seed name is equal", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: shoot})).To(BeFalse())
			})

			It("should return true because seed name changed", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.SeedName = pointer.String("new-seed")

				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return true", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should return true", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeTrue())
			})
		})
	})
})
