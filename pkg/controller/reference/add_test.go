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

package reference_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	. "github.com/gardener/gardener/pkg/controller/reference"
)

var _ = Describe("Add", func() {
	var reconciler *Reconciler

	BeforeEach(func() {
		reconciler = &Reconciler{
			ReferenceChangedPredicate: func(oldObj, newObj client.Object) bool {
				return oldObj.GetName() == "foo" && newObj.GetName() == "bar"
			},
		}
	})

	Describe("ShootPredicate", func() {
		var (
			p   predicate.Predicate
			obj *corev1.Pod
		)

		BeforeEach(func() {
			p = reconciler.Predicate()
			obj = &corev1.Pod{}
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			Context("obj has no deletion timestamp", func() {
				It("should return false because predicate function evaluates to false", func() {
					Expect(p.Update(event.UpdateEvent{ObjectNew: obj, ObjectOld: obj})).To(BeFalse())
				})

				It("should return true because predicate function evaluates to true", func() {
					oldObj := obj.DeepCopy()
					oldObj.Name = "foo"
					obj.Name = "bar"
					Expect(p.Update(event.UpdateEvent{ObjectNew: obj, ObjectOld: oldObj})).To(BeTrue())
				})
			})

			Context("obj has deletion timestamp", func() {
				BeforeEach(func() {
					obj.DeletionTimestamp = &metav1.Time{}
				})

				It("should return false because obj contains finalizer", func() {
					oldObj := obj.DeepCopy()
					obj.Finalizers = append(obj.Finalizers, "gardener")
					Expect(p.Update(event.UpdateEvent{ObjectNew: obj, ObjectOld: oldObj})).To(BeFalse())
				})

				It("should return false because obj does not contain finalizer", func() {
					Expect(p.Update(event.UpdateEvent{ObjectNew: obj, ObjectOld: obj})).To(BeTrue())
				})
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
