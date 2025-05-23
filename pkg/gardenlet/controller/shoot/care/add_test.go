// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/care"
	"github.com/gardener/gardener/pkg/utils/test"
	mockworkqueue "github.com/gardener/gardener/third_party/mock/client-go/util/workqueue"
)

var _ = Describe("Add", func() {
	var (
		reconciler *Reconciler
		shoot      *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		reconciler = &Reconciler{
			SeedName: "shoot",
			Config: gardenletconfigv1alpha1.GardenletConfiguration{
				Controllers: &gardenletconfigv1alpha1.GardenletControllerConfiguration{
					ShootCare: &gardenletconfigv1alpha1.ShootCareControllerConfiguration{
						SyncPeriod: &metav1.Duration{Duration: time.Minute},
					},
				},
			},
		}
		shoot = &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: "shoot", Namespace: "namespace"}}
	})

	Describe("#EventHandler", func() {
		var (
			ctx   = context.TODO()
			hdlr  handler.EventHandler
			queue *mockworkqueue.MockTypedRateLimitingInterface[reconcile.Request]
			req   reconcile.Request
		)

		BeforeEach(func() {
			hdlr = reconciler.EventHandler()
			queue = mockworkqueue.NewMockTypedRateLimitingInterface[reconcile.Request](gomock.NewController(GinkgoT()))
			req = reconcile.Request{NamespacedName: types.NamespacedName{Name: shoot.Name, Namespace: shoot.Namespace}}
		})

		It("should enqueue the object for Create events according to the calculated duration", func() {
			DeferCleanup(test.WithVar(&RandomDurationWithMetaDuration, func(max *metav1.Duration) time.Duration {
				return max.Duration
			}))
			queue.EXPECT().AddAfter(req, reconciler.Config.Controllers.ShootCare.SyncPeriod.Duration)

			hdlr.Create(ctx, event.CreateEvent{Object: shoot}, queue)
		})

		It("should enqueue the object for Update events", func() {
			queue.EXPECT().Add(req)

			hdlr.Update(ctx, event.UpdateEvent{ObjectNew: shoot, ObjectOld: shoot}, queue)
		})

		It("should not enqueue the object for Delete events", func() {
			hdlr.Delete(ctx, event.DeleteEvent{Object: shoot}, queue)
		})

		It("should not enqueue the object for Generic events", func() {
			hdlr.Generic(ctx, event.GenericEvent{Object: shoot}, queue)
		})
	})

	Describe("#ShootPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.ShootPredicate()
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because new object is no shoot", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is no shoot", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot})).To(BeFalse())
			})

			It("should return false because last operation is nil on old shoot", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: shoot, ObjectNew: shoot})).To(BeFalse())
			})

			It("should return false because last operation is nil on new shoot", func() {
				oldShoot := shoot.DeepCopy()
				oldShoot.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldShoot, ObjectNew: shoot})).To(BeFalse())
			})

			It("should return false because last operation type is 'Delete' on old shoot", func() {
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				oldShoot := shoot.DeepCopy()
				oldShoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeDelete
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldShoot, ObjectNew: shoot})).To(BeFalse())
			})

			It("should return false because last operation type is 'Delete' on new shoot", func() {
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeDelete
				oldShoot := shoot.DeepCopy()
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldShoot, ObjectNew: shoot})).To(BeFalse())
			})

			It("should return false because last operation type is not 'Processing' on old shoot", func() {
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
				shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
				oldShoot := shoot.DeepCopy()
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldShoot, ObjectNew: shoot})).To(BeFalse())
			})

			It("should return false because last operation type is not 'Succeeded' on new shoot", func() {
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
				shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateProcessing
				oldShoot := shoot.DeepCopy()
				oldShoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateProcessing
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldShoot, ObjectNew: shoot})).To(BeFalse())
			})

			It("should return true because last operation type is 'Succeeded' on new shoot", func() {
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
				shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
				oldShoot := shoot.DeepCopy()
				oldShoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateProcessing
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldShoot, ObjectNew: shoot})).To(BeTrue())
			})

			It("should return false when the seed name is unchanged in the Shoot spec", func() {
				shoot.Status.SeedName = ptr.To("test-seed")
				oldShoot := shoot.DeepCopy()
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldShoot, ObjectNew: shoot})).To(BeFalse())
			})

			It("should return false when the seed name is changed in the Shoot spec", func() {
				shoot.Status.SeedName = ptr.To("test-seed")
				oldShoot := shoot.DeepCopy()
				shoot.Status.SeedName = ptr.To("test-seed1")
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldShoot, ObjectNew: shoot})).To(BeFalse())
			})

			It("should return true when seed gets assigned to shoot", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Status.SeedName = ptr.To("test-seed")
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldShoot, ObjectNew: shoot})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return false", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeFalse())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})
})
