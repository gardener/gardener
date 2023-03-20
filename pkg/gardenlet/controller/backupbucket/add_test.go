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

package backupbucket_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/backupbucket"
)

var _ = Describe("Add", func() {
	var (
		reconciler   *Reconciler
		backupBucket *gardencorev1beta1.BackupBucket
		bucketName   = "bucket"
	)

	BeforeEach(func() {
		reconciler = &Reconciler{
			SeedName: "seed",
		}

		backupBucket = &gardencorev1beta1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: bucketName,
			},
			Spec: gardencorev1beta1.BackupBucketSpec{
				SeedName: pointer.String("seed"),
			},
		}
	})

	Describe("#SeedNamePredicate", func() {
		var (
			p predicate.Predicate
		)

		BeforeEach(func() {
			p = reconciler.SeedNamePredicate()
		})

		It("should return true because the backupbucket belongs to this seed", func() {
			Expect(p.Create(event.CreateEvent{Object: backupBucket})).To(BeTrue())
			Expect(p.Update(event.UpdateEvent{ObjectNew: backupBucket})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: backupBucket})).To(BeTrue())
			Expect(p.Generic(event.GenericEvent{Object: backupBucket})).To(BeTrue())
		})

		It("should return false because the backupbucket doesn't belong to this seed", func() {
			backupBucket.Spec.SeedName = pointer.String("some-other-seed")

			Expect(p.Create(event.CreateEvent{Object: backupBucket})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: backupBucket})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: backupBucket})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: backupBucket})).To(BeFalse())
		})
	})

	Describe("#MapExtensionBackupBucketToBackupBucket", func() {
		var (
			ctx                   = context.TODO()
			log                   = logr.Discard()
			extensionBackupBucket *extensionsv1alpha1.BackupBucket
		)

		BeforeEach(func() {
			extensionBackupBucket = &extensionsv1alpha1.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name: bucketName,
				},
			}
		})

		It("should return a request with the core.gardener.cloud/v1beta1.BackupBucket name", func() {
			Expect(reconciler.MapExtensionBackupBucketToCoreBackupBucket(ctx, log, nil, extensionBackupBucket)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: extensionBackupBucket.Name}},
			))
		})
	})
})
