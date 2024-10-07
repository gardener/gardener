// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mapper_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/pkg/controllerutils/mapper"
	mockcache "github.com/gardener/gardener/third_party/mock/controller-runtime/cache"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
)

var _ = Describe("EnqueueMapped", func() {
	Describe("#EnqueueRequestsFrom", func() {
		var (
			ctx     = context.TODO()
			mapper  Mapper
			logger  logr.Logger
			handler handler.EventHandler
			ctrl    *gomock.Controller
			mgr     *mockmanager.MockManager
			cache   *mockcache.MockCache

			queue   workqueue.TypedRateLimitingInterface[reconcile.Request]
			secret1 *corev1.Secret
			secret2 *corev1.Secret
		)

		BeforeEach(func() {
			mapper = MapFunc(func(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
				return []reconcile.Request{
					requestWithSuffix(obj, "1"),
					requestWithSuffix(obj, "2"),
				}
			})

			logger = logr.Discard()
			ctrl = gomock.NewController(GinkgoT())
			cache = mockcache.NewMockCache(ctrl)
			mgr = mockmanager.NewMockManager(ctrl)
			mgr.EXPECT().GetCache().Return(cache)

			queue = workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			secret1 = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: "namespace"}}
			secret2 = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: "namespace"}}
		})

		Describe("#Create", func() {
			It("should work map and enqueue", func() {
				handler = EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper, 0, logger)
				handler.Create(ctx, event.CreateEvent{Object: secret1}, queue)
				expectItems(queue, secret1)
			})
		})

		Describe("#Delete", func() {
			It("should work map and enqueue", func() {
				handler = EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper, 0, logger)
				handler.Delete(ctx, event.DeleteEvent{Object: secret1}, queue)
				expectItems(queue, secret1)
			})
		})

		Describe("#Generic", func() {
			It("should work map and enqueue", func() {
				handler = EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper, 0, logger)
				handler.Generic(ctx, event.GenericEvent{Object: secret1}, queue)
				expectItems(queue, secret1)
			})
		})

		Context("UpdateWithOldAndNew", func() {
			BeforeEach(func() {
				handler = EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper, UpdateWithOldAndNew, logger)
			})

			Describe("#Update", func() {
				It("should work map and enqueue", func() {
					handler.Update(ctx, event.UpdateEvent{ObjectOld: secret1, ObjectNew: secret2}, queue)
					expectItems(queue, secret1, secret2)
				})
			})
		})

		Context("UpdateWithNew", func() {
			BeforeEach(func() {
				handler = EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper, UpdateWithNew, logger)
			})

			Describe("#Update", func() {
				It("should work map and enqueue", func() {
					handler.Update(ctx, event.UpdateEvent{ObjectOld: secret1, ObjectNew: secret2}, queue)
					expectItems(queue, secret2)
				})
			})
		})

		Context("UpdateWithOld", func() {
			BeforeEach(func() {
				handler = EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper, UpdateWithOld, logger)
			})

			Describe("#Update", func() {
				It("should work map and enqueue", func() {
					handler.Update(ctx, event.UpdateEvent{ObjectOld: secret1, ObjectNew: secret2}, queue)
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

func expectItems(queue workqueue.TypedRateLimitingInterface[reconcile.Request], objects ...client.Object) {
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
