// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

	Describe("Predicate", func() {
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

				It("should return true because obj does not contain finalizer", func() {
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
