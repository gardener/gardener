// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenletlifecycle_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/gardenletlifecycle"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Add", func() {
	var reconciler *Reconciler

	BeforeEach(func() {
		reconciler = &Reconciler{}
	})

	Describe("#EventHandler", func() {
		var (
			ctx   = context.Background()
			hdlr  handler.TypedEventHandler[client.Object, Request]
			queue *test.FakeQueue[Request]
		)

		BeforeEach(func() {
			hdlr = reconciler.EventHandler()
			queue = &test.FakeQueue[Request]{}
		})

		It("should correctly enqueue Seeds", func() {
			obj := &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: "seed"}}

			hdlr.Create(ctx, event.CreateEvent{Object: obj}, queue)

			Expect(queue.Added).To(ConsistOf(Request{
				Request:           reconcile.Request{NamespacedName: types.NamespacedName{Name: "seed"}},
				IsSelfHostedShoot: false,
			}))
		})

		It("should correctly enqueue Shoots", func() {
			obj := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: "shoot", Namespace: "shoot-namespace"}}

			hdlr.Create(ctx, event.CreateEvent{Object: obj}, queue)

			Expect(queue.Added).To(ConsistOf(Request{
				Request:           reconcile.Request{NamespacedName: types.NamespacedName{Name: "shoot", Namespace: "shoot-namespace"}},
				IsSelfHostedShoot: true,
			}))
		})

		It("should not enqueue the object for Update events", func() {
			hdlr.Update(ctx, event.UpdateEvent{}, queue)

			Expect(queue.Added).To(BeEmpty())
		})

		It("should not enqueue the object for Delete events", func() {
			hdlr.Delete(ctx, event.DeleteEvent{}, queue)

			Expect(queue.Added).To(BeEmpty())
		})

		It("should not enqueue the object for Generic events", func() {
			hdlr.Generic(ctx, event.GenericEvent{}, queue)

			Expect(queue.Added).To(BeEmpty())
		})
	})
})
