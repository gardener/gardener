// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clusterfinalizer_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/clusterfinalizer"
)

var _ = Describe("Add", func() {
	var (
		ctx   context.Context
		queue workqueue.TypedRateLimitingInterface[reconcile.Request]
	)

	BeforeEach(func() {
		ctx = context.Background()
		queue = workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	})

	Describe("#ControllerInstallationEventHandlerForSeed", func() {
		var hdlr handler.EventHandler

		BeforeEach(func() {
			hdlr = ControllerInstallationEventHandlerForSeed()
		})

		It("should enqueue the seed on delete when seedRef is set", func() {
			controllerInstallation := &gardencorev1beta1.ControllerInstallation{
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					SeedRef: &corev1.ObjectReference{Name: "seed-1"},
				},
			}

			hdlr.Delete(ctx, event.DeleteEvent{Object: controllerInstallation}, queue)

			Expect(queue.Len()).To(Equal(1))
			item, _ := queue.Get()
			Expect(item).To(Equal(reconcile.Request{NamespacedName: types.NamespacedName{Name: "seed-1"}}))
		})

		It("should not enqueue on delete when seedRef is nil", func() {
			controllerInstallation := &gardencorev1beta1.ControllerInstallation{}

			hdlr.Delete(ctx, event.DeleteEvent{Object: controllerInstallation}, queue)

			Expect(queue.Len()).To(Equal(0))
		})

		It("should enqueue the seed on update when seedRef was cleared", func() {
			oldControllerInstallation := &gardencorev1beta1.ControllerInstallation{
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					SeedRef: &corev1.ObjectReference{Name: "seed-1"},
				},
			}
			newControllerInstallation := &gardencorev1beta1.ControllerInstallation{}

			hdlr.Update(ctx, event.UpdateEvent{ObjectOld: oldControllerInstallation, ObjectNew: newControllerInstallation}, queue)

			Expect(queue.Len()).To(Equal(1))
			item, _ := queue.Get()
			Expect(item).To(Equal(reconcile.Request{NamespacedName: types.NamespacedName{Name: "seed-1"}}))
		})

		It("should not enqueue on update when seedRef was not cleared", func() {
			oldControllerInstallation := &gardencorev1beta1.ControllerInstallation{
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					SeedRef: &corev1.ObjectReference{Name: "seed-1"},
				},
			}
			newControllerInstallation := &gardencorev1beta1.ControllerInstallation{
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					SeedRef: &corev1.ObjectReference{Name: "seed-1"},
				},
			}

			hdlr.Update(ctx, event.UpdateEvent{ObjectOld: oldControllerInstallation, ObjectNew: newControllerInstallation}, queue)

			Expect(queue.Len()).To(Equal(0))
		})

		It("should not enqueue on update when old seedRef is nil", func() {
			oldControllerInstallation := &gardencorev1beta1.ControllerInstallation{}
			newControllerInstallation := &gardencorev1beta1.ControllerInstallation{}

			hdlr.Update(ctx, event.UpdateEvent{ObjectOld: oldControllerInstallation, ObjectNew: newControllerInstallation}, queue)

			Expect(queue.Len()).To(Equal(0))
		})
	})

	Describe("#ControllerInstallationEventHandlerForShoot", func() {
		var hdlr handler.EventHandler

		BeforeEach(func() {
			hdlr = ControllerInstallationEventHandlerForShoot()
		})

		It("should enqueue the shoot on delete when shootRef is set", func() {
			controllerInstallation := &gardencorev1beta1.ControllerInstallation{
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					ShootRef: &corev1.ObjectReference{Name: "shoot-1", Namespace: "garden-project"},
				},
			}

			hdlr.Delete(ctx, event.DeleteEvent{Object: controllerInstallation}, queue)

			Expect(queue.Len()).To(Equal(1))
			item, _ := queue.Get()
			Expect(item).To(Equal(reconcile.Request{NamespacedName: types.NamespacedName{Name: "shoot-1", Namespace: "garden-project"}}))
		})

		It("should not enqueue on delete when shootRef is nil", func() {
			controllerInstallation := &gardencorev1beta1.ControllerInstallation{}

			hdlr.Delete(ctx, event.DeleteEvent{Object: controllerInstallation}, queue)

			Expect(queue.Len()).To(Equal(0))
		})

		It("should not enqueue on update events", func() {
			oldControllerInstallation := &gardencorev1beta1.ControllerInstallation{
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					ShootRef: &corev1.ObjectReference{Name: "shoot-1", Namespace: "garden-project"},
				},
			}
			newControllerInstallation := &gardencorev1beta1.ControllerInstallation{}

			hdlr.Update(ctx, event.UpdateEvent{ObjectOld: oldControllerInstallation, ObjectNew: newControllerInstallation}, queue)

			Expect(queue.Len()).To(Equal(0))
		})
	})
})
