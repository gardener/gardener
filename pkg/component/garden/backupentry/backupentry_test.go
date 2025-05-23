// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/component/garden/backupentry"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mocktime "github.com/gardener/gardener/third_party/mock/go/time"
)

var _ = Describe("BackupEntry", func() {
	var (
		ctrl *gomock.Controller

		ctx              context.Context
		c                client.Client
		expected         *gardencorev1beta1.BackupEntry
		values           *Values
		log              logr.Logger
		defaultDepWaiter Interface

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
		log = logr.Discard()

		s := runtime.NewScheme()
		Expect(gardencorev1beta1.AddToScheme(s)).NotTo(HaveOccurred())

		c = fake.NewClientBuilder().WithScheme(s).Build()

		values = &Values{
			Name:           name,
			Namespace:      namespace,
			ShootPurpose:   &shootPurpose,
			OwnerReference: ownerRef,
			SeedName:       &seedName,
			BucketName:     bucketName,
		}

		expected = &gardencorev1beta1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Annotations: map[string]string{
					v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
					v1beta1constants.ShootPurpose:      string(shootPurpose),
				},
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
			expected.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.GardenerOperationReconcile

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
			expected.ResourceVersion = "2"
			expected.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.GardenerOperationReconcile

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
			Expect(defaultDepWaiter.Wait(ctx)).To(MatchError(ContainSubstring("error during reconciliation: Some error")), "BackupEntry indicates error")
		})

		It("should return no error when is ready", func() {
			expected.Status.LastError = nil
			expected.Annotations = map[string]string{}
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

		It("should change the BucketName of the BackupEntry", func() {
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
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expected.Namespace, Name: expected.Name}, migrated)).To(Succeed())
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
		It("should not return error when it's not found", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
		})

		It("should not return error when it's deleted successfully", func() {
			Expect(c.Create(ctx, expected)).To(Succeed(), "creating BackupEntry succeeds")
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error when it's already removed", func() {
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(Succeed())
		})

		It("should return error when it's not deleted successfully", func() {
			Expect(c.Create(ctx, expected)).To(Succeed(), "creating backupentry succeeds")
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(MatchError(ContainSubstring("resource namespace/be still exists")))
		})
	})

	Describe("#Get", func() {
		It("should return error if the backupentry does not exist", func() {
			_, err := defaultDepWaiter.Get(ctx)
			Expect(err).To(HaveOccurred())
		})

		It("should return the retrieved backupentry and save it locally", func() {
			Expect(c.Create(ctx, expected)).To(Succeed())
			backupEntry, err := defaultDepWaiter.Get(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(backupEntry).To(Equal(expected))
			Expect(defaultDepWaiter.GetActualBucketName()).To(Equal(expected.Spec.BucketName))
		})
	})

	Describe("#SetForceDeletionAnnotation", func() {
		It("should not do anything if backupentry does not exist", func() {
			Expect(defaultDepWaiter.SetForceDeletionAnnotation(ctx)).To(Succeed())
		})

		It("should set the force-deletion annotation on the backupentry", func() {
			modified := expected.DeepCopy()

			Expect(c.Create(ctx, expected)).To(Succeed())
			Expect(defaultDepWaiter.SetForceDeletionAnnotation(ctx)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Namespace: modified.Namespace, Name: modified.Name}, modified)).To(Succeed())
			Expect(modified.Annotations["backupentry.core.gardener.cloud/force-deletion"]).To(Equal("true"))
		})
	})
})
