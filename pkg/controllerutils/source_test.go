// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/pkg/controllerutils"
)

var _ = Describe("Source", func() {
	Describe("#EnqueueOnce", func() {
		var (
			ctx   = context.Background()
			queue workqueue.RateLimitingInterface
		)

		BeforeEach(func() {
			queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
		})

		It("should enqueue an empty request", func() {
			Expect(EnqueueOnce(ctx, nil, queue)).To(Succeed())
			Expect(queue.Len()).To(Equal(1))

			item, v := queue.Get()
			Expect(item).To(Equal(reconcile.Request{}))
			Expect(v).To(BeFalse())
		})
	})

	Describe("#HandleOnce", func() {
		var (
			ctx          = context.Background()
			eventHandler = handler.Funcs{
				CreateFunc: func(_ context.Context, _ event.CreateEvent, queue workqueue.RateLimitingInterface) {
					queue.Add(reconcile.Request{})
				},
			}
			queue workqueue.RateLimitingInterface
		)

		BeforeEach(func() {
			queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
		})

		It("should enqueue an empty request", func() {
			Expect(HandleOnce(ctx, eventHandler, queue)).To(Succeed())
			Expect(queue.Len()).To(Equal(1))

			item, v := queue.Get()
			Expect(item).To(Equal(reconcile.Request{}))
			Expect(v).To(BeFalse())
		})
	})
})
