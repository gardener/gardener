// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry_test

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/extensions/backupentry"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/third_party/mock/go/time"
)

var _ = Describe("#BackupEntry", func() {
	var (
		ctrl *gomock.Controller

		ctx              context.Context
		c                client.Client
		empty            *extensionsv1alpha1.BackupEntry
		expected         *extensionsv1alpha1.BackupEntry
		values           *backupentry.Values
		log              logr.Logger
		fakeErr          error
		defaultDepWaiter component.DeployMigrateWaiter

		fakeClock *testclock.FakeClock
		mockNow   *mocktime.MockNow

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
		fakeClock = testclock.NewFakeClock(time.Now())

		ctx = context.TODO()
		log = logr.Discard()
		fakeErr = fmt.Errorf("some random error")

		s := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(s)).To(Succeed())

		c = fake.NewClientBuilder().WithScheme(s).Build()

		values = &backupentry.Values{
			Name:                       name,
			Type:                       providerType,
			ProviderConfig:             providerConfig,
			Region:                     region,
			SecretRef:                  secretRef,
			BucketName:                 bucketName,
			BackupBucketProviderStatus: backupBucketProviderStatus,
		}

		empty = &extensionsv1alpha1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}

		expected = &extensionsv1alpha1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
					v1beta1constants.GardenerTimestamp: fakeClock.Now().UTC().Format(time.RFC3339Nano),
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

		defaultDepWaiter = backupentry.New(log, c, fakeClock, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		BeforeEach(func() {
			expected.ResourceVersion = "1"
		})

		It("should create correct BackupEntry", func() {
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

		It("should return error if we haven't observed the latest timestamp annotation", func() {
			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("Patch object")
			patch := client.MergeFrom(expected.DeepCopy())
			expected.Status.LastError = nil
			// remove operation annotation, add old timestamp annotation
			expected.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: fakeClock.Now().Add(-time.Millisecond).UTC().Format(time.RFC3339Nano),
			}
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(c.Patch(ctx, expected, patch)).To(Succeed(), "patching backupentry succeeds")

			By("Wait")
			Expect(defaultDepWaiter.Wait(ctx)).NotTo(Succeed(), "backupentry indicates error")
		})

		It("should return no error when it's ready", func() {
			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("Patch object")
			patch := client.MergeFrom(expected.DeepCopy())
			expected.Status.LastError = nil
			// remove operation annotation, add up-to-date timestamp annotation
			expected.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: fakeClock.Now().UTC().Format(time.RFC3339Nano),
			}
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: metav1.Time{Time: fakeClock.Now().UTC().Add(time.Second)},
			}
			Expect(c.Patch(ctx, expected, patch)).To(Succeed(), "patching backupentry succeeds")

			By("Wait")
			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed(), "backupentry is ready")
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
			defer test.WithVars(
				&extensions.TimeNow, mockNow.Do,
				&gardenerutils.TimeNow, mockNow.Do,
			)()

			mockNow.EXPECT().Do().Return(fakeClock.Now().UTC()).AnyTimes()
			mc := mockclient.NewMockClient(ctrl)

			expected = empty.DeepCopy()
			expected.SetAnnotations(map[string]string{
				v1beta1constants.ConfirmationDeletion: "true",
				v1beta1constants.GardenerTimestamp:    fakeClock.Now().UTC().Format(time.RFC3339Nano),
			})

			// add deletion confirmation and timestamp annotation
			mc.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.BackupEntry{}), gomock.Any()).Return(nil)
			mc.EXPECT().Delete(ctx, expected).Times(1).Return(fakeErr)

			defaultDepWaiter = backupentry.New(log, mc, fakeClock, &backupentry.Values{Name: name}, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
			Expect(defaultDepWaiter.Destroy(ctx)).To(MatchError(fakeErr))
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error when it's already removed", func() {
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(Succeed())
		})

		It("should return error when it's not deleted successfully", func() {
			expected.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}

			Expect(c.Create(ctx, expected)).To(Succeed(), "creating backupentry succeeds")
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(MatchError(ContainSubstring("Some error")))
		})
	})

	Describe("#Restore", func() {
		var (
			state      = &runtime.RawExtension{Raw: []byte(`{"dummy":"state"}`)}
			shootState *gardencorev1beta1.ShootState
		)

		BeforeEach(func() {
			shootState = &gardencorev1beta1.ShootState{
				Spec: gardencorev1beta1.ShootStateSpec{
					Extensions: []gardencorev1beta1.ExtensionResourceState{
						{
							Name:  &expected.Name,
							Kind:  extensionsv1alpha1.BackupEntryResource,
							State: state,
						},
					},
				},
			}
		})

		It("should properly restore the BackupEntry state if it exists", func() {
			// NB(timebertt): such tests with mocks are ridiculously hard to adapt to refactoring changes.
			// Let's **please** just stop writing such tests with mocks and use a fake client or envtest instead.
			// Testing with mocks does not only assert that the tested unit fulfills its task but also
			// asserts that specific calls are made in order to fulfill its task. However, we/the caller don't
			// care about what helper funcs are used internally or whether it uses update or patch to fulfill
			// the task, as long as the result is what we expect (which is what should be asserted instead).
			defer test.WithVars(
				&extensions.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(fakeClock.Now().UTC()).AnyTimes()

			mc := mockclient.NewMockClient(ctrl)
			mockStatusWriter := mockclient.NewMockStatusWriter(ctrl)

			mc.EXPECT().Status().Return(mockStatusWriter)

			mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(empty), gomock.AssignableToTypeOf(empty)).
				Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("backupentries"), name))

			// deploy with wait-for-state annotation
			obj := expected.DeepCopy()
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/operation", "wait-for-state")
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/timestamp", fakeClock.Now().UTC().Format(time.RFC3339Nano))
			obj.TypeMeta = metav1.TypeMeta{}
			mc.EXPECT().Create(ctx, test.HasObjectKeyOf(obj)).
				DoAndReturn(func(_ context.Context, actual client.Object, _ ...client.CreateOption) error {
					Expect(actual).To(DeepEqual(obj))
					return nil
				})

			// restore state
			expectedWithState := obj.DeepCopy()
			expectedWithState.Status.State = state
			test.EXPECTStatusPatch(ctx, mockStatusWriter, expectedWithState, obj, types.MergePatchType)

			// annotate with restore annotation
			expectedWithRestore := expectedWithState.DeepCopy()
			metav1.SetMetaDataAnnotation(&expectedWithRestore.ObjectMeta, "gardener.cloud/operation", "restore")
			test.EXPECTPatch(ctx, mc, expectedWithRestore, expectedWithState, types.MergePatchType)

			Expect(backupentry.New(log, mc, fakeClock, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond).Restore(ctx, shootState)).To(Succeed())
		})
	})

	Describe("#Migrate", func() {
		It("should migrate the resource", func() {
			defer test.WithVars(
				&extensions.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(fakeClock.Now().UTC()).AnyTimes()
			mc := mockclient.NewMockClient(ctrl)

			expectedCopy := empty.DeepCopy()
			metav1.SetMetaDataAnnotation(&expectedCopy.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationMigrate)
			metav1.SetMetaDataAnnotation(&expectedCopy.ObjectMeta, v1beta1constants.GardenerTimestamp, fakeClock.Now().UTC().Format(time.RFC3339Nano))
			test.EXPECTPatch(ctx, mc, expectedCopy, empty, types.MergePatchType)

			defaultDepWaiter = backupentry.New(log, mc, fakeClock, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
			Expect(defaultDepWaiter.Migrate(ctx)).To(Succeed())
		})

		It("should not return error if resource does not exist", func() {
			defer test.WithVars(
				&extensions.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(fakeClock.Now().UTC()).AnyTimes()
			mc := mockclient.NewMockClient(ctrl)

			expectedCopy := empty.DeepCopy()
			metav1.SetMetaDataAnnotation(&expectedCopy.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationMigrate)
			metav1.SetMetaDataAnnotation(&expectedCopy.ObjectMeta, v1beta1constants.GardenerTimestamp, fakeClock.Now().UTC().Format(time.RFC3339Nano))
			test.EXPECTPatch(ctx, mc, expectedCopy, empty, types.MergePatchType)

			defaultDepWaiter = backupentry.New(log, mc, fakeClock, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
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
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(MatchError(ContainSubstring("to be successfully migrated")))
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
