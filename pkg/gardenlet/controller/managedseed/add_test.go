// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseed_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/managedseed"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Add", func() {
	var (
		ctx        = context.TODO()
		fakeClient client.Client
		reconciler *Reconciler
		p          predicate.Predicate
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		reconciler = &Reconciler{
			GardenClient:          fakeClient,
			GardenNamespaceGarden: v1beta1constants.GardenNamespace,
		}
	})

	Describe("#SeedPredicate", func() {
		var (
			seed *gardencorev1beta1.Seed
		)

		BeforeEach(func() {
			seed = &gardencorev1beta1.Seed{}
			p = reconciler.SeedPredicate()
		})

		It("should return true", func() {
			seed.Labels = map[string]string{
				"name.seed.gardener.cloud/foo": "true",
				"name.seed.gardener.cloud/bar": "true",
			}

			Expect(p.Create(event.TypedCreateEvent[client.Object]{Object: seed})).To(BeTrue())
			Expect(p.Update(event.TypedUpdateEvent[client.Object]{ObjectNew: seed})).To(BeTrue())
			Expect(p.Delete(event.TypedDeleteEvent[client.Object]{Object: seed})).To(BeTrue())
			Expect(p.Generic(event.TypedGenericEvent[client.Object]{Object: seed})).To(BeTrue())
		})

		It("should return true for self-hosted shoot clusters with a single name label", func() {
			seed.Labels = map[string]string{
				"name.seed.gardener.cloud/root":                 "true",
				"seed.gardener.cloud/self-hosted-shoot-cluster": "true",
			}

			Expect(p.Create(event.TypedCreateEvent[client.Object]{Object: seed})).To(BeTrue())
			Expect(p.Update(event.TypedUpdateEvent[client.Object]{ObjectNew: seed})).To(BeTrue())
			Expect(p.Delete(event.TypedDeleteEvent[client.Object]{Object: seed})).To(BeTrue())
			Expect(p.Generic(event.TypedGenericEvent[client.Object]{Object: seed})).To(BeTrue())
		})

		It("should return false", func() {
			Expect(p.Create(event.TypedCreateEvent[client.Object]{Object: seed})).To(BeFalse())
			Expect(p.Update(event.TypedUpdateEvent[client.Object]{ObjectNew: seed})).To(BeFalse())
			Expect(p.Delete(event.TypedDeleteEvent[client.Object]{Object: seed})).To(BeFalse())
			Expect(p.Generic(event.TypedGenericEvent[client.Object]{Object: seed})).To(BeFalse())
		})
	})

	Describe("#EnqueueWithJitterDelay", func() {
		var (
			hdlr           handler.EventHandler
			queue          *test.FakeQueue[reconcile.Request]
			obj            *seedmanagementv1alpha1.ManagedSeed
			req            reconcile.Request
			cfg            gardenletconfigv1alpha1.GardenletConfiguration
			randomDuration = 10 * time.Millisecond
		)

		BeforeEach(func() {
			cfg = gardenletconfigv1alpha1.GardenletConfiguration{
				Controllers: &gardenletconfigv1alpha1.GardenletControllerConfiguration{
					ManagedSeed: &gardenletconfigv1alpha1.ManagedSeedControllerConfiguration{
						SyncJitterPeriod: &metav1.Duration{Duration: 50 * time.Millisecond},
					},
				},
			}

			hdlr = (&Reconciler{Config: cfg}).EnqueueWithJitterDelay()
			queue = &test.FakeQueue[reconcile.Request]{}
			obj = &seedmanagementv1alpha1.ManagedSeed{ObjectMeta: metav1.ObjectMeta{Name: "managedseed", Namespace: "namespace"}}
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
			cfg.Controllers.ManagedSeed.JitterUpdates = ptr.To(false)
			obj.Generation = 2
			obj.Status.ObservedGeneration = 1
			hdlr = (&Reconciler{Config: cfg}).EnqueueWithJitterDelay()
			hdlr.Create(ctx, event.CreateEvent{Object: obj}, queue)

			Expect(queue.Added).To(ConsistOf(req))
		})

		It("should enqueue the object with random delay for Create events when generation changed and  jitterUpdates is set to true", func() {
			cfg.Controllers.ManagedSeed.JitterUpdates = ptr.To(true)
			obj.Generation = 2
			obj.Status.ObservedGeneration = 1
			hdlr = (&Reconciler{Config: cfg}).EnqueueWithJitterDelay()
			hdlr.Create(ctx, event.CreateEvent{Object: obj}, queue)

			Expect(queue.AddedAfter).To(ConsistOf(test.AddAfterArgs[reconcile.Request]{Item: req, Duration: randomDuration}))
		})

		It("should enqueue the object with random delay for Create events when there is no change in generation", func() {
			cfg.Controllers.ManagedSeed.JitterUpdates = ptr.To(false)
			obj.Generation = 2
			obj.Status.ObservedGeneration = 2
			hdlr = (&Reconciler{Config: cfg}).EnqueueWithJitterDelay()
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
			cfg.Controllers.ManagedSeed.JitterUpdates = ptr.To(false)
			obj.Generation = 2
			obj.Status.ObservedGeneration = 1
			hdlr = (&Reconciler{Config: cfg}).EnqueueWithJitterDelay()
			hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: obj}, queue)

			Expect(queue.Added).To(ConsistOf(req))
		})

		It("should enqueue the object with random delay for Update events when jitterUpdates is set to true", func() {
			cfg.Controllers.ManagedSeed.JitterUpdates = ptr.To(true)
			obj.Generation = 2
			obj.Status.ObservedGeneration = 1
			hdlr = (&Reconciler{Config: cfg}).EnqueueWithJitterDelay()
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
