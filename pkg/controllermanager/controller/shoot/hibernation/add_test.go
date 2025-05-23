// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hibernation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/shoot/hibernation"
)

var _ = Describe("Add", func() {
	var (
		reconciler *Reconciler
		shoot      *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		reconciler = &Reconciler{}
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
			Spec: gardencorev1beta1.ShootSpec{
				Hibernation: &gardencorev1beta1.Hibernation{
					Schedules: []gardencorev1beta1.HibernationSchedule{{
						Start: ptr.To("00 20 * * 1,2,3,4,5"),
					}},
				},
			},
		}
	})

	Describe("#ShootPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.ShootPredicate()
		})

		Describe("#Create", func() {
			It("should return false because object is no shoot", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeFalse())
			})

			It("should return false because shoot has no hibernation schedules", func() {
				shoot.Spec.Hibernation = nil
				Expect(p.Create(event.CreateEvent{Object: shoot})).To(BeFalse())
			})

			It("should return true because shoot has hibernation schedules", func() {
				Expect(p.Create(event.CreateEvent{Object: shoot})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because new object is no shoot", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is no shoot", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot})).To(BeFalse())
			})

			It("should return false because hibernation schedules are equal", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: shoot})).To(BeFalse())
			})

			It("should return false because hibernation schedules are not equal but new shoot does not have any schedule anymore", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Hibernation = nil
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeFalse())
			})

			It("should return true because hibernation schedules are equal and new shoot still has schedules", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Hibernation.Schedules[0].Start = ptr.To("00 20 * * 1,2,3,4,5,6,7")
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
			})
		})
	})
})
