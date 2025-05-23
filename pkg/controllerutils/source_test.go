// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/pkg/controllerutils"
)

var _ = Describe("Source", func() {
	var (
		ctx   = context.Background()
		queue workqueue.TypedRateLimitingInterface[reconcile.Request]
	)

	BeforeEach(func() {
		queue = workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	})

	Describe("#EnqueueOnce", func() {
		It("should enqueue an empty request", func() {
			Expect(EnqueueOnce(ctx, queue)).To(Succeed())
			Expect(queue.Len()).To(Equal(1))

			item, v := queue.Get()
			Expect(item).To(Equal(reconcile.Request{}))
			Expect(v).To(BeFalse())
		})
	})

	Describe("#EnqueueAnonymously", func() {
		var (
			obj = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "known"}}
		)

		assert := func() {
			ExpectWithOffset(1, queue.Len()).To(Equal(1))

			item, v := queue.Get()
			ExpectWithOffset(1, item).To(Equal(reconcile.Request{}))
			ExpectWithOffset(1, v).To(BeFalse())
		}

		Describe("#Create", func() {
			It("should enqueue anonymously", func() {
				EnqueueAnonymously.Create(ctx, event.CreateEvent{Object: obj}, queue)
				assert()
			})
		})

		Describe("#Update", func() {
			It("should enqueue anonymously", func() {
				EnqueueAnonymously.Update(ctx, event.UpdateEvent{ObjectOld: obj, ObjectNew: obj}, queue)
				assert()
			})
		})

		Describe("#Delete", func() {
			It("should enqueue anonymously", func() {
				EnqueueAnonymously.Delete(ctx, event.DeleteEvent{Object: obj}, queue)
				assert()
			})
		})

		Describe("#Generic", func() {
			It("should enqueue anonymously", func() {
				EnqueueAnonymously.Generic(ctx, event.GenericEvent{Object: obj}, queue)
				assert()
			})
		})
	})
})
