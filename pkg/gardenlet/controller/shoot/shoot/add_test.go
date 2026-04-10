// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/shoot"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/shoot/helper"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Add", func() {
	var (
		ctx = context.TODO()
		log logr.Logger
		cl  clock.Clock
		cfg gardenletconfigv1alpha1.GardenletConfiguration
	)

	BeforeEach(func() {
		log = logr.Discard()

		cl = testclock.NewFakeClock(time.Now())
		cfg = gardenletconfigv1alpha1.GardenletConfiguration{
			Controllers: &gardenletconfigv1alpha1.GardenletControllerConfiguration{
				Shoot: &gardenletconfigv1alpha1.ShootControllerConfiguration{
					SyncPeriod: &metav1.Duration{Duration: time.Hour},
				},
			},
		}
	})

	Describe("#EventHandler", func() {
		var (
			hdlr  handler.EventHandler
			queue *test.FakeQueue[reconcile.Request]
			obj   *gardencorev1beta1.Shoot
			req   reconcile.Request
		)

		BeforeEach(func() {
			hdlr = (&Reconciler{
				Config: cfg,
				Clock:  cl,
			}).EventHandler(log)
			queue = &test.FakeQueue[reconcile.Request]{}
			obj = &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: "shoot", Namespace: "namespace"}}
			req = reconcile.Request{NamespacedName: types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}}
		})

		It("should enqueue the object for Create events according to the calculated duration", func() {
			duration := time.Minute
			DeferCleanup(test.WithVar(&CalculateControllerInfos, func(*gardencorev1beta1.Seed, *gardencorev1beta1.Shoot, clock.Clock, gardenletconfigv1alpha1.ShootControllerConfiguration) helper.ControllerInfos {
				return helper.ControllerInfos{
					EnqueueAfter: duration,
				}
			}))

			hdlr.Create(ctx, event.CreateEvent{Object: obj}, queue)

			Expect(queue.AddedAfter).To(ConsistOf(test.AddAfterArgs[reconcile.Request]{Item: req, Duration: duration}))
		})

		It("should enqueue the object for Update events", func() {
			hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: obj}, queue)

			Expect(queue.Added).To(ConsistOf(req))
		})

		It("should forget the backoff and enqueue the object for Update events setting the deletionTimestamp", func() {
			objOld := obj.DeepCopy()
			now := metav1.Now()
			obj.SetDeletionTimestamp(&now)
			hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: objOld}, queue)

			Expect(queue.Forgotten).To(ConsistOf(req))
			Expect(queue.Added).To(ConsistOf(req))
		})

		It("should not enqueue the object for Delete events", func() {
			hdlr.Delete(ctx, event.DeleteEvent{Object: obj}, queue)

			Expect(queue.Added).To(BeEmpty())
			Expect(queue.AddedAfter).To(BeEmpty())
		})

		It("should not enqueue the object for Generic events", func() {
			hdlr.Generic(ctx, event.GenericEvent{Object: obj}, queue)

			Expect(queue.Added).To(BeEmpty())
			Expect(queue.AddedAfter).To(BeEmpty())
		})
	})
})
