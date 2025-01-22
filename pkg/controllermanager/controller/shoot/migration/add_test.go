// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package migration_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/shoot/migration"
)

var _ = Describe("Add", func() {
	var reconciler *Reconciler

	BeforeEach(func() {
		reconciler = &Reconciler{}
	})

	Describe("#ShootPredicate", func() {
		var (
			predicate predicate.Predicate
			shoot     *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			predicate = reconciler.ShootPredicate()
			shoot = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: ptr.To("seed-1"),
				},
				Status: gardencorev1beta1.ShootStatus{
					SeedName: ptr.To("seed-1"),
				},
			}
		})

		Describe("#Create", func() {
			It("should return false because shoot is not being migration", func() {
				Expect(predicate.Create(event.CreateEvent{Object: shoot})).To(BeFalse())
			})

			It("should return true because shoot needs migration", func() {
				shoot.Spec.SeedName = ptr.To("seed-2")

				Expect(predicate.Create(event.CreateEvent{Object: shoot})).To(BeTrue())
			})

			It("should return true because constraint is still present after migration", func() {
				shoot.Status.Constraints = []gardencorev1beta1.Condition{
					{Type: "ReadyForMigration"},
				}

				Expect(predicate.Create(event.CreateEvent{Object: shoot})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because shoot is not being migration", func() {
				Expect(predicate.Update(event.UpdateEvent{ObjectNew: shoot})).To(BeFalse())
			})

			It("should return true because shoot needs migration", func() {
				shoot.Spec.SeedName = ptr.To("seed-2")

				Expect(predicate.Update(event.UpdateEvent{ObjectNew: shoot})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should always return false", func() {
				Expect(predicate.Delete(event.DeleteEvent{})).To(BeFalse())
			})
		})

		Describe("#Generic", func() {
			It("should always return false", func() {
				Expect(predicate.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})
})
