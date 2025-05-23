// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucketscheck_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/seed/backupbucketscheck"
)

var _ = Describe("Add", func() {
	var (
		reconciler   *Reconciler
		backupBucket *gardencorev1beta1.BackupBucket
	)

	BeforeEach(func() {
		reconciler = &Reconciler{}
		backupBucket = &gardencorev1beta1.BackupBucket{
			Spec: gardencorev1beta1.BackupBucketSpec{
				SeedName: ptr.To("seed"),
			},
		}
	})

	Describe("BackupBucketPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.BackupBucketPredicate()
		})

		Describe("#Create", func() {
			It("should return false because object is no BackupBucket", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeFalse())
			})

			It("should return false because seed name is not set", func() {
				backupBucket.Spec.SeedName = nil
				Expect(p.Create(event.CreateEvent{Object: backupBucket})).To(BeFalse())
			})

			It("should return true because seed name is set", func() {
				Expect(p.Create(event.CreateEvent{Object: backupBucket})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because object is no BackupBucket", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is no BackupBucket", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: backupBucket})).To(BeFalse())
			})

			It("should return false because seed name is not set", func() {
				backupBucket.Spec.SeedName = nil
				Expect(p.Update(event.UpdateEvent{ObjectNew: backupBucket, ObjectOld: backupBucket})).To(BeFalse())
			})

			It("should return false because last error did not change", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: backupBucket, ObjectOld: backupBucket})).To(BeFalse())
			})

			It("should return true because last error was set", func() {
				oldBackupBucket := backupBucket.DeepCopy()
				backupBucket.Status.LastError = &gardencorev1beta1.LastError{}
				Expect(p.Update(event.UpdateEvent{ObjectNew: backupBucket, ObjectOld: oldBackupBucket})).To(BeTrue())
			})

			It("should return true because last error was removed", func() {
				backupBucket.Status.LastError = &gardencorev1beta1.LastError{}
				oldBackupBucket := backupBucket.DeepCopy()
				backupBucket.Status.LastError = nil
				Expect(p.Update(event.UpdateEvent{ObjectNew: backupBucket, ObjectOld: oldBackupBucket})).To(BeTrue())
			})

			It("should return true because last error was changed", func() {
				backupBucket.Status.LastError = &gardencorev1beta1.LastError{Description: "foo"}
				oldBackupBucket := backupBucket.DeepCopy()
				backupBucket.Status.LastError.Description = "bar"
				Expect(p.Update(event.UpdateEvent{ObjectNew: backupBucket, ObjectOld: oldBackupBucket})).To(BeTrue())
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

	Describe("#MapBackupBucketToSeed", func() {
		ctx := context.TODO()

		It("should do nothing if the object is no BackupBucket", func() {
			Expect(reconciler.MapBackupBucketToSeed(ctx, &corev1.Secret{})).To(BeEmpty())
		})

		It("should do nothing if seed name is not set", func() {
			backupBucket.Spec.SeedName = nil
			Expect(reconciler.MapBackupBucketToSeed(ctx, backupBucket)).To(BeEmpty())
		})

		It("should map the BackupBucket to the Seed", func() {
			Expect(reconciler.MapBackupBucketToSeed(ctx, backupBucket)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: *backupBucket.Spec.SeedName}},
			))
		})
	})
})
