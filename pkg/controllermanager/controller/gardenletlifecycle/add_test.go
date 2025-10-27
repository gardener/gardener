// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenletlifecycle_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/gardenletlifecycle"
	mockworkqueue "github.com/gardener/gardener/third_party/mock/client-go/util/workqueue"
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
			queue *mockworkqueue.MockTypedRateLimitingInterface[Request]
		)

		BeforeEach(func() {
			hdlr = reconciler.EventHandler()
			queue = mockworkqueue.NewMockTypedRateLimitingInterface[Request](gomock.NewController(GinkgoT()))
		})

		It("should correctly enqueue Seeds", func() {
			obj := &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: "seed"}}

			queue.EXPECT().Add(Request{
				Request:           reconcile.Request{NamespacedName: types.NamespacedName{Name: "seed"}},
				IsSelfHostedShoot: false,
			})

			hdlr.Create(ctx, event.CreateEvent{Object: obj}, queue)
		})

		It("should correctly enqueue Shoots", func() {
			obj := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: "shoot", Namespace: "shoot-namespace"}}

			queue.EXPECT().Add(Request{
				Request:           reconcile.Request{NamespacedName: types.NamespacedName{Name: "shoot", Namespace: "shoot-namespace"}},
				IsSelfHostedShoot: true,
			})

			hdlr.Create(ctx, event.CreateEvent{Object: obj}, queue)
		})

		It("should not enqueue the object for Update events", func() {
			hdlr.Update(ctx, event.UpdateEvent{}, queue)
		})

		It("should not enqueue the object for Delete events", func() {
			hdlr.Delete(ctx, event.DeleteEvent{}, queue)
		})

		It("should not enqueue the object for Generic events", func() {
			hdlr.Generic(ctx, event.GenericEvent{}, queue)
		})
	})
})
