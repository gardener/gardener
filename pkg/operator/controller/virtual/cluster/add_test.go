// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cluster_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllertest"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	. "github.com/gardener/gardener/pkg/operator/controller/virtual/cluster"
)

var _ = Describe("Add", func() {
	var (
		ctx        context.Context
		reconciler *Reconciler
		restConfig *rest.Config
	)

	BeforeEach(func() {
		ctx = context.Background()
		reconciler = &Reconciler{}
		restConfig = &rest.Config{Host: "http://api.foo.bar"}
	})

	Describe("EventHandler", func() {
		var (
			eventHandler handler.TypedEventHandler[*rest.Config, Request]
			queue        workqueue.TypedRateLimitingInterface[Request]
		)

		BeforeEach(func() {
			eventHandler = reconciler.EventHandler()
			queue = &controllertest.TypedQueue[Request]{
				TypedInterface: workqueue.NewTyped[Request](),
			}
		})

		Describe("#Generic", func() {
			It("should add the REST config", func() {
				eventHandler.Generic(ctx, event.TypedGenericEvent[*rest.Config]{Object: restConfig}, queue)
				item, shutdown := queue.Get()
				ExpectWithOffset(1, shutdown).To(BeFalse())
				ExpectWithOffset(1, item.RESTConfig).To(Equal(restConfig))
			})
		})

		Describe("#Create", func() {
			It("should do nothing", func() {
				eventHandler.Create(ctx, event.TypedCreateEvent[*rest.Config]{Object: restConfig}, queue)
				Expect(queue.Len()).To(Equal(0))
			})
		})

		Describe("#Update", func() {
			It("should do nothing", func() {
				eventHandler.Update(ctx, event.TypedUpdateEvent[*rest.Config]{ObjectNew: restConfig}, queue)
				Expect(queue.Len()).To(Equal(0))
			})
		})

		Describe("#Delete", func() {
			It("should do nothing", func() {
				eventHandler.Delete(ctx, event.TypedDeleteEvent[*rest.Config]{Object: restConfig}, queue)
				Expect(queue.Len()).To(Equal(0))
			})
		})
	})
})
