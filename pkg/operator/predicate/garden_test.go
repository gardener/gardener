//  SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
//  SPDX-License-Identifier: Apache-2.0

package predicate_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	operatorpredicate "github.com/gardener/gardener/pkg/operator/predicate"
)

var _ = Describe("Garden", func() {
	var (
		garden *operatorv1alpha1.Garden
		p      predicate.Predicate
	)

	BeforeEach(func() {
		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name: "garden",
			},
		}
	})

	Describe("#GardenCreatedOrReconciledSuccessfully", func() {
		BeforeEach(func() {
			p = operatorpredicate.GardenCreatedOrReconciledSuccessfully()
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because new object is no garden", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is no garden", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: garden})).To(BeFalse())
			})

			It("should return false because last operation is nil on old garden", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: garden, ObjectNew: garden})).To(BeFalse())
			})

			It("should return false because last operation is nil on new garden", func() {
				oldShoot := garden.DeepCopy()
				oldShoot.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldShoot, ObjectNew: garden})).To(BeFalse())
			})

			It("should return false because last operation type is 'Delete' on old garden", func() {
				garden.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				oldShoot := garden.DeepCopy()
				oldShoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeDelete
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldShoot, ObjectNew: garden})).To(BeFalse())
			})

			It("should return false because last operation type is 'Delete' on new garden", func() {
				garden.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				garden.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeDelete
				oldShoot := garden.DeepCopy()
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldShoot, ObjectNew: garden})).To(BeFalse())
			})

			It("should return false because last operation type is not 'Processing' on old garden", func() {
				garden.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				garden.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
				garden.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
				oldShoot := garden.DeepCopy()
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldShoot, ObjectNew: garden})).To(BeFalse())
			})

			It("should return false because last operation type is not 'Succeeded' on new garden", func() {
				garden.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				garden.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
				garden.Status.LastOperation.State = gardencorev1beta1.LastOperationStateProcessing
				oldShoot := garden.DeepCopy()
				oldShoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateProcessing
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldShoot, ObjectNew: garden})).To(BeFalse())
			})

			It("should return true because last operation type is 'Succeeded' on new garden", func() {
				garden.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				garden.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
				garden.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
				oldShoot := garden.DeepCopy()
				oldShoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateProcessing
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldShoot, ObjectNew: garden})).To(BeTrue())
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

	Describe("#GardenDeletionTriggered", func() {
		BeforeEach(func() {
			p = operatorpredicate.GardenDeletionTriggered()
		})

		Describe("#Create", func() {
			It("should return false", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeFalse())
			})
		})

		Describe("#Update", func() {
			It("should return false when deletion timestamp is not set", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: garden, ObjectOld: garden})).To(BeFalse())
			})

			It("should return false when deletion timestamp is already set", func() {
				garden.DeletionTimestamp = &metav1.Time{}
				Expect(p.Update(event.UpdateEvent{ObjectNew: garden, ObjectOld: garden})).To(BeFalse())
			})

			It("should return true when deletion timestamp was just set", func() {
				gardenOld := garden.DeepCopy()
				garden.DeletionTimestamp = &metav1.Time{}
				Expect(p.Update(event.UpdateEvent{ObjectNew: garden, ObjectOld: gardenOld})).To(BeTrue())
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
