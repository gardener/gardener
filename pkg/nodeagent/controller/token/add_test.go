// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package token_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllertest"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/pkg/nodeagent/controller/token"
)

var _ = Describe("Add", func() {
	var (
		ctx        context.Context
		reconciler *Reconciler
		secret     *corev1.Secret
	)

	BeforeEach(func() {
		ctx = context.Background()
		reconciler = &Reconciler{}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "namespace",
			},
		}
	})

	Describe("EventHandler", func() {
		var (
			eventHandler handler.TypedEventHandler[*corev1.Secret, reconcile.Request]
			queue        workqueue.TypedRateLimitingInterface[reconcile.Request]
		)

		BeforeEach(func() {
			eventHandler = reconciler.EventHandler()
			queue = &controllertest.TypedQueue[reconcile.Request]{
				TypedInterface: workqueue.NewTyped[reconcile.Request](),
			}
		})

		Describe("#Generic", func() {
			It("should add the request", func() {
				eventHandler.Generic(ctx, event.TypedGenericEvent[*corev1.Secret]{Object: secret}, queue)
				item, shutdown := queue.Get()
				ExpectWithOffset(1, shutdown).To(BeFalse())
				ExpectWithOffset(1, item.Name).To(Equal(secret.Name))
				ExpectWithOffset(1, item.Namespace).To(Equal(secret.Namespace))
			})
		})

		Describe("#Create", func() {
			It("should do nothing", func() {
				eventHandler.Create(ctx, event.TypedCreateEvent[*corev1.Secret]{Object: secret}, queue)
				Expect(queue.Len()).To(Equal(0))
			})
		})

		Describe("#Update", func() {
			It("should do nothing", func() {
				eventHandler.Update(ctx, event.TypedUpdateEvent[*corev1.Secret]{ObjectNew: secret}, queue)
				Expect(queue.Len()).To(Equal(0))
			})
		})

		Describe("#Delete", func() {
			It("should do nothing", func() {
				eventHandler.Delete(ctx, event.TypedDeleteEvent[*corev1.Secret]{Object: secret}, queue)
				Expect(queue.Len()).To(Equal(0))
			})
		})
	})
})
