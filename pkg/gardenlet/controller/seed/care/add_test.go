// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/seed/care"
)

var _ = Describe("Add", func() {
	var (
		reconciler      *Reconciler
		seed            *gardencorev1beta1.Seed
		managedResource *resourcesv1alpha1.ManagedResource
	)

	BeforeEach(func() {
		reconciler = &Reconciler{
			SeedName: "seed",
		}
		seed = &gardencorev1beta1.Seed{}
		managedResource = &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: "garden"}}
	})

	Describe("#SeedPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.SeedPredicate()
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because new object is no seed", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is no seed", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: seed})).To(BeFalse())
			})

			It("should return false because last operation is nil on old shoot", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: seed, ObjectNew: seed})).To(BeFalse())
			})

			It("should return false because last operation is nil on new seed", func() {
				oldSeed := seed.DeepCopy()
				oldSeed.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSeed, ObjectNew: seed})).To(BeFalse())
			})

			It("should return false because last operation type is 'Delete' on old seed", func() {
				seed.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				oldSeed := seed.DeepCopy()
				oldSeed.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeDelete
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSeed, ObjectNew: seed})).To(BeFalse())
			})

			It("should return false because last operation type is 'Delete' on new seed", func() {
				seed.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				seed.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeDelete
				oldSeed := seed.DeepCopy()
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSeed, ObjectNew: seed})).To(BeFalse())
			})

			It("should return false because last operation type is not 'Processing' on old seed", func() {
				seed.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				seed.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
				seed.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
				oldSeed := seed.DeepCopy()
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSeed, ObjectNew: seed})).To(BeFalse())
			})

			It("should return false because last operation type is not 'Succeeded' on new seed", func() {
				seed.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				seed.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
				seed.Status.LastOperation.State = gardencorev1beta1.LastOperationStateProcessing
				oldSeed := seed.DeepCopy()
				oldSeed.Status.LastOperation.State = gardencorev1beta1.LastOperationStateProcessing
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSeed, ObjectNew: seed})).To(BeFalse())
			})

			It("should return true because last operation type is 'Succeeded' on new seed", func() {
				seed.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				seed.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
				seed.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
				oldSeed := seed.DeepCopy()
				oldSeed.Status.LastOperation.State = gardencorev1beta1.LastOperationStateProcessing
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSeed, ObjectNew: seed})).To(BeTrue())
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

	Describe("#IsSystemComponent", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.IsSystemComponent()
		})

		It("should return false because the label is not present", func() {
			Expect(p.Create(event.CreateEvent{Object: managedResource})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: managedResource})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: managedResource})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: managedResource})).To(BeFalse())
		})

		It("should return true because the label is present", func() {
			managedResource.Labels = map[string]string{"gardener.cloud/role": "seed-system-component"}

			Expect(p.Create(event.CreateEvent{Object: managedResource})).To(BeTrue())
			Expect(p.Update(event.UpdateEvent{ObjectNew: managedResource})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: managedResource})).To(BeTrue())
			Expect(p.Generic(event.GenericEvent{Object: managedResource})).To(BeTrue())
		})
	})

	Describe("#MapManagedResourceToSeed", func() {
		It("should return a request with the seed name", func() {
			Expect(reconciler.MapManagedResourceToSeed(context.TODO(), nil)).To(ConsistOf(reconcile.Request{NamespacedName: types.NamespacedName{Name: reconciler.SeedName}}))
		})
	})
})
