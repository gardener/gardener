// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils_test

import (
	"context"
	"time"

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

	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	. "github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/test"
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

	Describe("#EnqueueWithJitterDelay", func() {
		var (
			hdlr             handler.EventHandler
			queue            *test.FakeQueue[reconcile.Request]
			obj              *seedmanagementv1alpha1.ManagedSeed
			req              reconcile.Request
			randomDuration   = 10 * time.Millisecond
			syncJitterPeriod *metav1.Duration
		)

		BeforeEach(func() {
			syncJitterPeriod = &metav1.Duration{Duration: 50 * time.Millisecond}

			queue = &test.FakeQueue[reconcile.Request]{}
			obj = &seedmanagementv1alpha1.ManagedSeed{ObjectMeta: metav1.ObjectMeta{Name: "managedseed", Namespace: "namespace"}}
			hdlr = EnqueueWithJitterDelay(
				func(obj client.Object) (int64, bool) {
					ms, ok := obj.(*seedmanagementv1alpha1.ManagedSeed)
					if !ok {
						return 0, false
					}
					return ms.Status.ObservedGeneration, true
				},
				new(true),
				syncJitterPeriod,
			)
			req = reconcile.Request{NamespacedName: types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}}

			DeferCleanup(func() {
				test.WithVar(&RandomDurationWithMetaDuration, func(_ *metav1.Duration) time.Duration { return randomDuration })
			})
		})

		It("should enqueue the object without delay for Create events when deletion timestamp is set", func() {
			now := metav1.Now()
			obj.SetDeletionTimestamp(&now)
			hdlr.Create(ctx, event.CreateEvent{Object: obj}, queue)

			Expect(queue.Added).To(ConsistOf(req))
		})

		It("should enqueue the object without delay for Create events when generation is set to 1", func() {
			obj.Generation = 1
			hdlr.Create(ctx, event.CreateEvent{Object: obj}, queue)

			Expect(queue.Added).To(ConsistOf(req))
		})

		It("should enqueue the object without delay for Create events when generation changed and jitterudpates is set to false", func() {
			obj.Generation = 2
			obj.Status.ObservedGeneration = 1
			hdlr = EnqueueWithJitterDelay(
				func(obj client.Object) (int64, bool) {
					ms, ok := obj.(*seedmanagementv1alpha1.ManagedSeed)
					if !ok {
						return 0, false
					}
					return ms.Status.ObservedGeneration, true
				},
				new(false),
				syncJitterPeriod,
			)
			hdlr.Create(ctx, event.CreateEvent{Object: obj}, queue)

			Expect(queue.Added).To(ConsistOf(req))
		})

		It("should enqueue the object with random delay for Create events when generation changed and  jitterUpdates is set to true", func() {
			obj.Generation = 2
			obj.Status.ObservedGeneration = 1
			hdlr.Create(ctx, event.CreateEvent{Object: obj}, queue)

			Expect(queue.AddedAfter).To(ConsistOf(test.AddAfterArgs[reconcile.Request]{Item: req, Duration: randomDuration}))
		})

		It("should enqueue the object with random delay for Create events when there is no change in generation", func() {
			obj.Generation = 2
			obj.Status.ObservedGeneration = 2
			hdlr = EnqueueWithJitterDelay(
				func(obj client.Object) (int64, bool) {
					ms, ok := obj.(*seedmanagementv1alpha1.ManagedSeed)
					if !ok {
						return 0, false
					}
					return ms.Status.ObservedGeneration, true
				},
				new(false),
				syncJitterPeriod,
			)
			hdlr.Create(ctx, event.CreateEvent{Object: obj}, queue)

			Expect(queue.AddedAfter).To(ConsistOf(test.AddAfterArgs[reconcile.Request]{Item: req, Duration: randomDuration}))
		})

		It("should not enqueue the object for Update events when generation and observedGeneration are equal", func() {
			obj.Generation = 1
			obj.Status.ObservedGeneration = 1
			hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: obj}, queue)

			Expect(queue.Added).To(BeEmpty())
			Expect(queue.AddedAfter).To(BeEmpty())
		})

		It("should enqueue the object for Update events when deletion timestamp is set", func() {
			obj.Generation = 2
			obj.Status.ObservedGeneration = 1
			now := metav1.Now()
			obj.SetDeletionTimestamp(&now)
			hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: obj}, queue)

			Expect(queue.Added).To(ConsistOf(req))
		})

		It("should enqueue the object for Update events when generation is 1", func() {
			obj.Generation = 1
			obj.Status.ObservedGeneration = 0
			hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: obj}, queue)

			Expect(queue.Added).To(ConsistOf(req))
		})

		It("should enqueue the object for Update events when jitterUpdates is set to false", func() {
			obj.Generation = 2
			obj.Status.ObservedGeneration = 1
			hdlr = EnqueueWithJitterDelay(
				func(obj client.Object) (int64, bool) {
					ms, ok := obj.(*seedmanagementv1alpha1.ManagedSeed)
					if !ok {
						return 0, false
					}
					return ms.Status.ObservedGeneration, true
				},
				new(false),
				syncJitterPeriod,
			)

			hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: obj}, queue)

			Expect(queue.Added).To(ConsistOf(req))
		})

		It("should enqueue the object with random delay for Update events when jitterUpdates is set to true", func() {
			obj.Generation = 2
			obj.Status.ObservedGeneration = 1
			hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: obj}, queue)

			Expect(queue.AddedAfter).To(ConsistOf(test.AddAfterArgs[reconcile.Request]{Item: req, Duration: randomDuration}))
		})

		It("should enqueue the object for Delete events", func() {
			hdlr.Delete(ctx, event.DeleteEvent{Object: obj}, queue)

			Expect(queue.Added).To(ConsistOf(req))
		})

		It("should not enqueue the object for Generic events", func() {
			hdlr.Generic(ctx, event.GenericEvent{Object: obj}, queue)

			Expect(queue.Added).To(BeEmpty())
			Expect(queue.AddedAfter).To(BeEmpty())
		})
	})
})
