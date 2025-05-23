// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
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

	Describe("ShootUnassignedPredicate", func() {
		var (
			predicate predicate.Predicate
			shoot     *gardencorev1beta1.Shoot

			createEvent  event.CreateEvent
			updateEvent  event.UpdateEvent
			deleteEvent  event.DeleteEvent
			genericEvent event.GenericEvent
		)

		BeforeEach(func() {
			predicate = reconciler.ShootUnassignedPredicate()
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
				shoot.Spec.SeedName = ptr.To("seed")
			})

			It("should be false", func() {
				Expect(predicate.Create(createEvent)).To(BeFalse())
				Expect(predicate.Update(updateEvent)).To(BeFalse())
				Expect(predicate.Delete(deleteEvent)).To(BeFalse())
				Expect(predicate.Generic(genericEvent)).To(BeFalse())
			})
		})

		Context("shoot defines schedulerName", func() {
			Context("default-scheduler", func() {
				BeforeEach(func() {
					shoot.Spec.SchedulerName = ptr.To("default-scheduler")
				})

				It("should be true", func() {
					Expect(predicate.Create(createEvent)).To(BeTrue())
					Expect(predicate.Update(updateEvent)).To(BeTrue())
					Expect(predicate.Delete(deleteEvent)).To(BeTrue())
					Expect(predicate.Generic(genericEvent)).To(BeTrue())
				})
			})

			Context("arbitrary scheduler name", func() {
				BeforeEach(func() {
					shoot.Spec.SchedulerName = ptr.To("foo-scheduler")
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

	Describe("ShootSpecChangedPredicate", func() {
		var (
			predicate predicate.Predicate
			shootNew  *gardencorev1beta1.Shoot
			shootOld  *gardencorev1beta1.Shoot

			createEvent  event.CreateEvent
			updateEvent  event.UpdateEvent
			deleteEvent  event.DeleteEvent
			genericEvent event.GenericEvent
		)

		BeforeEach(func() {
			predicate = reconciler.ShootSpecChangedPredicate()
			shootOld = &gardencorev1beta1.Shoot{}
			shootNew = &gardencorev1beta1.Shoot{}

			createEvent = event.CreateEvent{
				Object: nil,
			}
			updateEvent = event.UpdateEvent{
				ObjectOld: shootOld,
				ObjectNew: shootNew,
			}
			deleteEvent = event.DeleteEvent{
				Object: nil,
			}
			genericEvent = event.GenericEvent{
				Object: nil,
			}
		})

		Context("shoot spec unchanged", func() {
			It("should be false", func() {
				Expect(predicate.Create(createEvent)).To(BeTrue())
				Expect(predicate.Update(updateEvent)).To(BeFalse())
				Expect(predicate.Delete(deleteEvent)).To(BeTrue())
				Expect(predicate.Generic(genericEvent)).To(BeTrue())
			})
		})

		Context("shoot spec changed", func() {
			BeforeEach(func() {
				shootNew.Spec.Tolerations = []gardencorev1beta1.Toleration{{Key: "foo"}}
			})

			It("should be true", func() {
				Expect(predicate.Create(createEvent)).To(BeTrue())
				Expect(predicate.Update(updateEvent)).To(BeTrue())
				Expect(predicate.Delete(deleteEvent)).To(BeTrue())
				Expect(predicate.Generic(genericEvent)).To(BeTrue())
			})
		})
	})
})
