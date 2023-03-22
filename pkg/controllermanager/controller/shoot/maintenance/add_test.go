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

package maintenance_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/shoot/maintenance"
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
			shoot = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Maintenance: &gardencorev1beta1.Maintenance{
						TimeWindow: &gardencorev1beta1.MaintenanceTimeWindow{},
					},
				},
			}
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

			It("should return false because there is neither maintain-now annotation nor time window change", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: shoot})).To(BeFalse())
			})

			It("should return false when there is maintain-now annotation only on old object", func() {
				oldShoot := shoot.DeepCopy()
				metav1.SetMetaDataAnnotation(&oldShoot.ObjectMeta, "gardener.cloud/operation", "maintain")
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeFalse())
			})

			It("should return false when there is maintain-now annotation on old and new object", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "gardener.cloud/operation", "maintain")
				oldShoot := shoot.DeepCopy()
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeFalse())
			})

			It("should return true when there is maintain-now annotation only on new object", func() {
				oldShoot := shoot.DeepCopy()
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "gardener.cloud/operation", "maintain")
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
			})

			It("should return true because there is time window change", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Maintenance.TimeWindow.Begin = "123"
				shoot.Spec.Maintenance.TimeWindow.End = "456"
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
