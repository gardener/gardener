// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package backupentry_test

import (
	"context"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/logger"
	mocktime "github.com/gardener/gardener/pkg/mock/go/time"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/backupentry"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("BackupEntry", func() {
	var (
		ctrl *gomock.Controller

		ctx              context.Context
		c                client.Client
		expected         *gardencorev1beta1.BackupEntry
		values           *Values
		log              logrus.FieldLogger
		defaultDepWaiter component.DeployMigrateWaiter

		mockNow *mocktime.MockNow
		now     time.Time

		name         = "be"
		namespace    = "namespace"
		shootPurpose = gardencorev1beta1.ShootPurposeDevelopment
		ownerRef     = metav1.NewControllerRef(&corev1.Namespace{}, corev1.SchemeGroupVersion.WithKind("Namespace"))

		seedName            = "seed"
		bucketName          = "bucket"
		differentBucketName = "different-bucketname"
		differentSeedName   = "different-seedname"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		mockNow = mocktime.NewMockNow(ctrl)

		ctx = context.TODO()
		log = logger.NewNopLogger()

		s := runtime.NewScheme()
		Expect(gardencorev1beta1.AddToScheme(s)).NotTo(HaveOccurred())

		c = fake.NewFakeClientWithScheme(s)

		values = &Values{
			Name:           name,
			Namespace:      namespace,
			ShootPurpose:   &shootPurpose,
			OwnerReference: ownerRef,
			SeedName:       &seedName,
			BucketName:     bucketName,
		}

		expected = &gardencorev1beta1.BackupEntry{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "BackupEntry",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Annotations: map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
					v1beta1constants.GardenerTimestamp: now.UTC().String(),
					v1beta1constants.ShootPurpose:      string(shootPurpose),
				},
				Finalizers:      []string{"gardener"},
				OwnerReferences: []metav1.OwnerReference{*ownerRef},
			},
			Spec: gardencorev1beta1.BackupEntrySpec{
				BucketName: bucketName,
				SeedName:   &seedName,
			},
		}

		defaultDepWaiter = New(log, c, values, time.Millisecond, 500*time.Millisecond)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		BeforeEach(func() {
			expected.ResourceVersion = "1"
		})

		It("should create correct BackupEntry (newly created)", func() {
			defer test.WithVars(&TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			actual := &gardencorev1beta1.BackupEntry{}
			Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, actual)).To(Succeed())

			Expect(actual).To(DeepEqual(expected))
		})

		It("should create correct BackupEntry (reconciling/updating)", func() {
			defer test.WithVars(&TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			existing := expected.DeepCopy()
			existing.ResourceVersion = ""
			existing.Spec.BucketName = differentBucketName
			existing.Spec.SeedName = &differentSeedName
			Expect(c.Create(ctx, existing)).To(Succeed(), "creating BackupEntry succeeds")

			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			actual := &gardencorev1beta1.BackupEntry{}
			Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, actual)).To(Succeed())

			expected.Spec.BucketName = differentBucketName
			expected.Spec.SeedName = &differentSeedName
			Expect(actual).To(DeepEqual(expected))
		})
	})

	Describe("#Wait", func() {
		It("should return error when it's not found", func() {
			Expect(defaultDepWaiter.Wait(ctx)).To(HaveOccurred())
		})

		It("should return error when it's not ready", func() {
			expected.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}

			Expect(c.Create(ctx, expected)).To(Succeed(), "creating BackupEntry succeeds")
			Expect(defaultDepWaiter.Wait(ctx)).To(MatchError(ContainSubstring("has not yet been reconciled")), "BackupEntry indicates error")
		})

		It("should return no error when is ready", func() {
			expected.Status.LastError = nil
			expected.ObjectMeta.Annotations = map[string]string{}
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}

			Expect(c.Create(ctx, expected)).To(Succeed(), "creating BackupEntry succeeds")
			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed(), "BackupEntry is ready, should not return an error")
		})
	})

	Describe("#Restore", func() {
		BeforeEach(func() {
			defaultDepWaiter = New(log, c, values, time.Millisecond, 500*time.Millisecond)
		})

		It("should correctly restore BackupEntry", func() {
			defer test.WithVars(&TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			existing := expected.DeepCopy()
			existing.ResourceVersion = ""
			existing.Spec.BucketName = differentBucketName
			existing.Spec.SeedName = &differentSeedName
			Expect(c.Create(ctx, existing)).To(Succeed(), "restoring BackupEntry succeeds")

			Expect(defaultDepWaiter.Restore(ctx, nil)).To(Succeed())

			actual := &gardencorev1beta1.BackupEntry{}
			Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, actual)).To(Succeed())

			expected.ResourceVersion = "2"
			expected.Spec.BucketName = differentBucketName
			expected.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.GardenerOperationRestore
			Expect(actual).To(DeepEqual(expected))
		})
	})

	Describe("#Migrate", func() {
		It("should return error when it's not found", func() {
			Expect(defaultDepWaiter.Migrate(ctx)).To(Succeed())
		})

		It("should correctly migrate the BackupEntry", func() {
			defer test.WithVars(&TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			existing := expected.DeepCopy()
			existing.ResourceVersion = ""
			existing.Spec.SeedName = nil
			Expect(c.Create(ctx, existing)).To(Succeed(), "creating BackupEntry succeeds")

			Expect(defaultDepWaiter.Migrate(ctx)).To(Succeed())

			actual := &gardencorev1beta1.BackupEntry{}
			Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, actual)).To(Succeed())

			expected.ResourceVersion = "2"
			expected.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.GardenerOperationMigrate
			Expect(actual).To(DeepEqual(expected))
		})
	})

	Describe("#WaitMigrate", func() {
		It("should return error when it's not found", func() {
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(HaveOccurred())
		})

		It("should return error when it's not ready", func() {
			migrated := expected.DeepCopy()

			expected.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateError,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(c.Create(ctx, expected)).To(Succeed(), "migrating BackupEntry succeeds")
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(HaveOccurred())
			Expect(c.Get(ctx, kutil.Key(expected.Namespace, expected.Name), migrated)).To(Succeed())
			Expect(migrated.Status.LastOperation).To(Equal(expected.Status.LastOperation))
		})

		It("should not return error when migrated successfully", func() {
			expected.Status.LastError = nil
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(c.Create(ctx, expected)).To(Succeed(), "migrating BackupEntry succeeds")
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(Succeed(), "BackupEntry is ready, should not return an error")
		})
	})

	Describe("#Destroy", func() {
		It("should be nil because it's not implemented", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).To(BeNil())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should be nil because it's not implemented", func() {
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(BeNil())
		})
	})
})
