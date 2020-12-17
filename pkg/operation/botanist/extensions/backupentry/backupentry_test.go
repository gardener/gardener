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
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/pkg/mock/go/time"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/backupentry"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("#BackupEntry", func() {
	var (
		ctrl *gomock.Controller

		ctx              context.Context
		c                client.Client
		expected         *extensionsv1alpha1.BackupEntry
		values           *backupentry.Values
		log              logrus.FieldLogger
		fakeErr          error
		defaultDepWaiter component.DeployMigrateWaiter

		mockNow *mocktime.MockNow
		now     time.Time

		name                       = "test-deploy"
		region                     = "region"
		bucketName                 = "bucketname"
		providerType               = "foo"
		providerConfig             = &runtime.RawExtension{Raw: []byte(`{"bar":"foo"}`)}
		backupBucketProviderStatus = &runtime.RawExtension{Raw: []byte(`{"foo":"bar"}`)}
		secretRef                  = corev1.SecretReference{
			Name:      "secretname",
			Namespace: "secretnamespace",
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		mockNow = mocktime.NewMockNow(ctrl)

		ctx = context.TODO()
		log = logger.NewNopLogger()
		fakeErr = fmt.Errorf("some random error")

		s := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(s)).To(Succeed())

		c = fake.NewFakeClientWithScheme(s)

		values = &backupentry.Values{
			Name:                       name,
			Type:                       providerType,
			ProviderConfig:             providerConfig,
			Region:                     region,
			SecretRef:                  secretRef,
			BucketName:                 bucketName,
			BackupBucketProviderStatus: backupBucketProviderStatus,
		}

		expected = &extensionsv1alpha1.BackupEntry{
			TypeMeta: metav1.TypeMeta{
				APIVersion: extensionsv1alpha1.SchemeGroupVersion.String(),
				Kind:       extensionsv1alpha1.BackupEntryResource,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
					v1beta1constants.GardenerTimestamp: now.UTC().String(),
				},
			},
			Spec: extensionsv1alpha1.BackupEntrySpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type:           providerType,
					ProviderConfig: providerConfig,
				},
				Region:                     region,
				SecretRef:                  secretRef,
				BucketName:                 bucketName,
				BackupBucketProviderStatus: backupBucketProviderStatus,
			},
		}

		defaultDepWaiter = backupentry.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		BeforeEach(func() {
			expected.ResourceVersion = "1"
		})

		It("should create correct BackupEntry", func() {
			defer test.WithVars(&backupentry.TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			actual := &extensionsv1alpha1.BackupEntry{}
			Expect(c.Get(ctx, client.ObjectKey{Name: name}, actual)).To(Succeed())

			Expect(actual).To(DeepEqual(expected))
		})
	})

	Describe("#Wait", func() {
		It("should return error when it's not found", func() {
			Expect(defaultDepWaiter.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
		})

		It("should return error when it's not ready", func() {
			expected.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}

			Expect(c.Create(ctx, expected)).To(Succeed(), "creating backupentry succeeds")
			Expect(defaultDepWaiter.Wait(ctx)).To(MatchError(ContainSubstring("error during reconciliation: Some error")), "backupentry indicates error")
		})

		It("should return no error when is ready", func() {
			expected.Status.LastError = nil
			expected.ObjectMeta.Annotations = map[string]string{}
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}

			Expect(c.Create(ctx, expected)).To(Succeed(), "creating backupentry succeeds")
			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed(), "backupentry is ready, should not return an error")
		})
	})

	Describe("#Destroy", func() {
		It("should not return error when it's not found", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
		})

		It("should not return error when it's deleted successfully", func() {
			Expect(c.Create(ctx, expected)).To(Succeed(), "adding pre-existing backupentry succeeds")
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
		})

		It("should return error when it's not deleted successfully", func() {
			defer test.WithVars(&common.TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			expected := extensionsv1alpha1.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Annotations: map[string]string{
						common.ConfirmationDeletion:        "true",
						v1beta1constants.GardenerTimestamp: now.UTC().String(),
					},
				}}

			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Get(ctx, kutil.Key(name), gomock.AssignableToTypeOf(&extensionsv1alpha1.BackupEntry{}))

			// add deletion confirmation and timestamp annotation
			mc.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.BackupEntry{})).Return(nil)
			mc.EXPECT().Delete(ctx, &expected).Times(1).Return(fakeErr)

			defaultDepWaiter = backupentry.New(log, mc, &backupentry.Values{Name: name}, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
			Expect(defaultDepWaiter.Destroy(ctx)).To(MatchError(fakeErr))
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error when it's already removed", func() {
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(Succeed())
		})
	})

	Describe("#Restore", func() {
		BeforeEach(func() {
			expected.ResourceVersion = "1"
		})

		It("should perform a normal deployment", func() {
			defer test.WithVars(&backupentry.TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			Expect(defaultDepWaiter.Restore(ctx, nil)).To(Succeed())

			actual := &extensionsv1alpha1.BackupEntry{}
			Expect(c.Get(ctx, client.ObjectKey{Name: name}, actual)).To(Succeed())

			Expect(actual).To(DeepEqual(expected))
		})
	})

	Describe("#Migrate", func() {
		It("should migrate the resource", func() {
			defer test.WithVars(
				&backupentry.TimeNow, mockNow.Do,
				&common.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			expectedCopy := expected.DeepCopy()
			expectedCopy.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.GardenerOperationMigrate

			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Get(ctx, kutil.Key(name), gomock.AssignableToTypeOf(&extensionsv1alpha1.BackupEntry{})).SetArg(2, *expected)
			mc.EXPECT().Patch(ctx, expectedCopy, gomock.AssignableToTypeOf(client.MergeFrom(expected)))

			defaultDepWaiter = backupentry.New(log, mc, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
			Expect(defaultDepWaiter.Migrate(ctx)).To(Succeed())
		})

		It("should not return error if resource does not exist", func() {
			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Get(ctx, kutil.Key(name), gomock.AssignableToTypeOf(&extensionsv1alpha1.BackupEntry{})).Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("BackupEntry"), expected.Name))

			defaultDepWaiter = backupentry.New(log, mc, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
			Expect(defaultDepWaiter.Migrate(ctx)).To(Succeed())
		})
	})

	Describe("#WaitMigrate", func() {
		It("should not return error when resource is missing", func() {
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(Succeed())
		})

		It("should return error if resource is not yet migrated successfully", func() {
			expected.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}

			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateError,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(c.Create(ctx, expected)).To(Succeed(), "creating BackupEntry succeeds")
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(MatchError(ContainSubstring("is not Migrate=Succeeded")))
		})

		It("should not return error if resource gets migrated successfully", func() {
			expected.Status.LastError = nil
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(c.Create(ctx, expected)).To(Succeed(), "creating BackupEntry succeeds")
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(Succeed(), "BackupEntry is ready, should not return an error")
		})
	})
})
