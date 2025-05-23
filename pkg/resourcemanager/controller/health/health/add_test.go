// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/pkg/resourcemanager/controller/health/health"
)

var _ = Describe("Add", func() {
	Describe("#EnqueueCreateAndUpdate", func() {
		var (
			ctx   = context.TODO()
			hdlr  handler.EventHandler
			queue workqueue.TypedRateLimitingInterface[reconcile.Request]
			obj   *corev1.Secret
		)

		BeforeEach(func() {
			hdlr = (&Reconciler{}).EnqueueCreateAndUpdate()
			queue = workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			obj = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: "namespace"}}
		})

		It("should enqueue the object for Create events", func() {
			hdlr.Create(ctx, event.CreateEvent{Object: obj}, queue)

			Expect(queue.Len()).To(Equal(1))
			item, v := queue.Get()
			Expect(item).To(Equal(reconcile.Request{NamespacedName: types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}}))
			Expect(v).To(BeFalse())
		})

		It("should enqueue the object for Update events", func() {
			hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: obj}, queue)

			Expect(queue.Len()).To(Equal(1))
			item, v := queue.Get()
			Expect(item).To(Equal(reconcile.Request{NamespacedName: types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}}))
			Expect(v).To(BeFalse())
		})

		It("should not enqueue the object for Delete events", func() {
			hdlr.Delete(ctx, event.DeleteEvent{Object: obj}, queue)

			Expect(queue.Len()).To(Equal(0))
		})

		It("should not enqueue the object for Generic events", func() {
			hdlr.Generic(ctx, event.GenericEvent{Object: obj}, queue)

			Expect(queue.Len()).To(Equal(0))
		})
	})
})
