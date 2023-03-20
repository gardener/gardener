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

package mapper_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	. "github.com/gardener/gardener/pkg/controllerutils/mapper"
)

var _ = Describe("EnqueueMapped", func() {
	Describe("#EnqueueRequestsFrom", func() {
		var (
			mapper  Mapper
			logger  logr.Logger
			handler handler.EventHandler

			queue   workqueue.RateLimitingInterface
			secret1 *corev1.Secret
			secret2 *corev1.Secret
		)

		BeforeEach(func() {
			mapper = MapFunc(func(ctx context.Context, log logr.Logger, reader client.Reader, obj client.Object) []reconcile.Request {
				return []reconcile.Request{
					requestWithSuffix(obj, "1"),
					requestWithSuffix(obj, "2"),
				}
			})

			logger = logr.Discard()
			handler = EnqueueRequestsFrom(mapper, 0, logger)

			_, ok := handler.(inject.Cache)
			Expect(ok).To(BeTrue())
			_, ok = handler.(inject.Stoppable)
			Expect(ok).To(BeTrue())
			_, ok = handler.(inject.Injector)
			Expect(ok).To(BeTrue())

			queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
			secret1 = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: "namespace"}}
			secret2 = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: "namespace"}}
		})

		Describe("#Create", func() {
			It("should work map and enqueue", func() {
				handler.Create(event.CreateEvent{Object: secret1}, queue)
				expectItems(queue, secret1)
			})
		})

		Describe("#Delete", func() {
			It("should work map and enqueue", func() {
				handler.Delete(event.DeleteEvent{Object: secret1}, queue)
				expectItems(queue, secret1)
			})
		})

		Describe("#Generic", func() {
			It("should work map and enqueue", func() {
				handler.Generic(event.GenericEvent{Object: secret1}, queue)
				expectItems(queue, secret1)
			})
		})

		Context("UpdateWithOldAndNew", func() {
			BeforeEach(func() {
				handler = EnqueueRequestsFrom(mapper, UpdateWithOldAndNew, logger)
			})

			Describe("#Update", func() {
				It("should work map and enqueue", func() {
					handler.Update(event.UpdateEvent{ObjectOld: secret1, ObjectNew: secret2}, queue)
					expectItems(queue, secret1, secret2)
				})
			})
		})

		Context("UpdateWithNew", func() {
			BeforeEach(func() {
				handler = EnqueueRequestsFrom(mapper, UpdateWithNew, logger)
			})

			Describe("#Update", func() {
				It("should work map and enqueue", func() {
					handler.Update(event.UpdateEvent{ObjectOld: secret1, ObjectNew: secret2}, queue)
					expectItems(queue, secret2)
				})
			})
		})

		Context("UpdateWithOld", func() {
			BeforeEach(func() {
				handler = EnqueueRequestsFrom(mapper, UpdateWithOld, logger)
			})

			Describe("#Update", func() {
				It("should work map and enqueue", func() {
					handler.Update(event.UpdateEvent{ObjectOld: secret1, ObjectNew: secret2}, queue)
					expectItems(queue, secret1)
				})
			})
		})
	})

	Describe("#ObjectListToRequests", func() {
		list := &corev1.SecretList{
			Items: []corev1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: "namespace1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: "namespace2"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: "namespace2"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: "namespace3"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: "namespace4"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: "namespace5"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: "namespace6"}},
			},
		}

		It("should return the correct requests w/p predicates", func() {
			Expect(ObjectListToRequests(list)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace1"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace2"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret2", Namespace: "namespace2"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace3"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace4"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace5"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace6"}},
			))
		})

		It("should return the correct requests w/ predicates", func() {
			var (
				predicate1 = func(o client.Object) bool { return o.GetNamespace() != "namespace3" }
				predicate2 = func(o client.Object) bool { return o.GetNamespace() != "namespace5" }
			)

			Expect(ObjectListToRequests(list, predicate1, predicate2)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace1"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace2"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret2", Namespace: "namespace2"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace4"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace6"}},
			))
		})
	})
})

func expectItems(queue workqueue.RateLimitingInterface, objects ...client.Object) {
	ExpectWithOffset(1, queue.Len()).To(Equal(len(objects) * 2))

	for _, obj := range objects {
		item, _ := queue.Get()
		ExpectWithOffset(1, item).To(Equal(requestWithSuffix(obj, "1")), "expected request with suffix 1")
		item, _ = queue.Get()
		ExpectWithOffset(1, item).To(Equal(requestWithSuffix(obj, "2")), "expected request with suffix 2")
	}
}

func requestWithSuffix(object client.Object, suffix string) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Name: object.GetName() + suffix, Namespace: object.GetNamespace() + suffix}}
}
