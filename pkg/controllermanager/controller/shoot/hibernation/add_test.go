// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package hibernation_test

import (
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/shoot/hibernation"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
						Start: pointer.String("00 20 * * 1,2,3,4,5"),
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
				shoot.Spec.Hibernation.Schedules[0].Start = pointer.String("00 20 * * 1,2,3,4,5,6,7")
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
			})
		})
	})

	Describe("#ShootEventHandler", func() {
		var (
			log   logr.Logger
			h     handler.EventHandler
			queue *fakeQueue
		)

		BeforeEach(func() {
			log = logr.Discard()
			h = reconciler.ShootEventHandler(log)
			queue = &fakeQueue{}
		})

		Describe("#CreateFunc", func() {
			It("should do nothing because object is nil", func() {
				h.Create(event.CreateEvent{}, queue)
				Expect(queue.Len()).To(BeZero())
			})

			It("should add the object to the queue", func() {
				h.Create(event.CreateEvent{Object: shoot}, queue)
				Expect(queue.Len()).To(Equal(1))
				Expect(queue.items[0]).To(Equal(reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      shoot.Name,
						Namespace: shoot.Namespace,
					},
				}))
			})
		})

		Describe("#UpdateFunc", func() {
			It("should do nothing because object is nil", func() {
				h.Update(event.UpdateEvent{}, queue)
				Expect(queue.Len()).To(BeZero())
			})

			It("should not add the object to the queue because schedules are not parseable", func() {
				shoot.Spec.Hibernation.Schedules[0].Start = pointer.String("not-parseable")
				h.Update(event.UpdateEvent{ObjectNew: shoot}, queue)
				Expect(queue.Len()).To(BeZero())
			})

			It("should add the object to the queue", func() {
				h.Update(event.UpdateEvent{ObjectNew: shoot}, queue)
				Expect(queue.Len()).To(Equal(1))
				Expect(queue.items[0].Namespace).To(Equal(shoot.Namespace))
				Expect(queue.items[0].Name).To(HavePrefix(shoot.Name + "-"))
				_, err := time.ParseDuration(strings.TrimPrefix(queue.items[0].Name, shoot.Name+"-"))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})

type fakeQueue struct {
	workqueue.RateLimitingInterface
	items []reconcile.Request
}

func (f *fakeQueue) Add(item interface{}) {
	f.items = append(f.items, item.(reconcile.Request))
}

func (f *fakeQueue) AddAfter(item interface{}, duration time.Duration) {
	i := item.(reconcile.Request)
	i.Name += "-" + duration.String()
	f.items = append(f.items, i)
}

func (f *fakeQueue) Len() int {
	return len(f.items)
}
