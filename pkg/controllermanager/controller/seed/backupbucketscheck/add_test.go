// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package backupbucketscheck_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
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
				SeedName: pointer.String("seed"),
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
		var (
			ctx        = context.TODO()
			log        logr.Logger
			fakeClient client.Client
		)

		BeforeEach(func() {
			log = logr.Discard()
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		})

		It("should do nothing if the object is no BackupBucket", func() {
			Expect(reconciler.MapBackupBucketToSeed(ctx, log, fakeClient, &corev1.Secret{})).To(BeEmpty())
		})

		It("should do nothing if seed name is not set", func() {
			backupBucket.Spec.SeedName = nil
			Expect(reconciler.MapBackupBucketToSeed(ctx, log, fakeClient, backupBucket)).To(BeEmpty())
		})

		It("should map the BackupBucket to the Seed", func() {
			Expect(reconciler.MapBackupBucketToSeed(ctx, log, fakeClient, backupBucket)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: *backupBucket.Spec.SeedName}},
			))
		})
	})
})
