// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/garden/backupbucket"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("BackupBucket", func() {
	var (
		ctx        = context.Background()
		log        = logr.Discard()
		fakeClient client.Client
		fakeClock  clock.Clock

		backupBucketName = "bucket-name"
		defaultRegion    = "default-region"
		providerType     = "provider"
		providerConfig   = &runtime.RawExtension{Raw: []byte(`{"foo":"bar"}`)}
		secretName       = "secret-name"
		secretNamespace  = "secret-namespace"
		backupConfig     *gardencorev1beta1.Backup

		deployer Interface
		values   *Values

		expectedBackupBucket *gardencorev1beta1.BackupBucket
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		fakeClock = testclock.NewFakeClock(time.Now())

		backupConfig = &gardencorev1beta1.Backup{
			Provider:       providerType,
			ProviderConfig: providerConfig,
			CredentialsRef: &corev1.ObjectReference{
				Name:      secretName,
				Namespace: secretNamespace,
			},
		}

		values = &Values{
			Name:          backupBucketName,
			Config:        backupConfig,
			DefaultRegion: defaultRegion,
			Clock:         fakeClock,
		}
		deployer = New(log, fakeClient, values, 50*time.Millisecond, 200*time.Millisecond)

		expectedBackupBucket = &gardencorev1beta1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{Name: backupBucketName},
			Spec: gardencorev1beta1.BackupBucketSpec{
				Provider: gardencorev1beta1.BackupBucketProvider{
					Type:   providerType,
					Region: defaultRegion,
				},
				ProviderConfig: providerConfig,
				SecretRef: corev1.SecretReference{
					Name:      secretName,
					Namespace: secretNamespace,
				},
			},
		}
	})

	Describe("#Deploy", func() {
		BeforeEach(func() {
			expectedBackupBucket.ResourceVersion = "1"
		})

		It("should create correct BackupBucket (newly created)", func() {
			Expect(deployer.Deploy(ctx)).To(Succeed())

			actual := &gardencorev1beta1.BackupBucket{ObjectMeta: metav1.ObjectMeta{Name: backupBucketName}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(actual), actual)).To(Succeed())

			metav1.SetMetaDataAnnotation(&expectedBackupBucket.ObjectMeta, "gardener.cloud/operation", "reconcile")
			metav1.SetMetaDataAnnotation(&expectedBackupBucket.ObjectMeta, "gardener.cloud/timestamp", fakeClock.Now().UTC().Format(time.RFC3339Nano))
			Expect(actual).To(DeepEqual(expectedBackupBucket))
		})

		It("should create correct BackupBucket (reconciling/updating)", func() {
			existing := expectedBackupBucket.DeepCopy()
			existing.ResourceVersion = ""
			existing.Spec.Provider.Type = "other-provider"
			existing.Spec.Provider.Region = "other-region"
			Expect(fakeClient.Create(ctx, existing)).To(Succeed())

			Expect(deployer.Deploy(ctx)).To(Succeed())

			actual := &gardencorev1beta1.BackupBucket{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(existing), actual)).To(Succeed())

			expectedBackupBucket.ResourceVersion = "2"
			expectedBackupBucket.Spec.Provider.Type = providerType
			expectedBackupBucket.Spec.Provider.Region = defaultRegion
			metav1.SetMetaDataAnnotation(&expectedBackupBucket.ObjectMeta, "gardener.cloud/operation", "reconcile")
			metav1.SetMetaDataAnnotation(&expectedBackupBucket.ObjectMeta, "gardener.cloud/timestamp", fakeClock.Now().UTC().Format(time.RFC3339Nano))
			Expect(actual).To(DeepEqual(expectedBackupBucket))
		})

		When("seed is present and region is overridden", func() {
			BeforeEach(func() {
				backupConfig.Region = ptr.To("overridden-region")
				values.Seed = &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: "seed"}}
			})

			It("should create correct BackupBucket (newly created)", func() {
				Expect(deployer.Deploy(ctx)).To(Succeed())

				actual := &gardencorev1beta1.BackupBucket{ObjectMeta: metav1.ObjectMeta{Name: backupBucketName}}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(actual), actual)).To(Succeed())

				expectedBackupBucket.OwnerReferences = []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Seed",
					Name:               "seed",
					BlockOwnerDeletion: ptr.To(true),
					Controller:         ptr.To(true),
				}}
				expectedBackupBucket.Spec.Provider.Region = "overridden-region"
				expectedBackupBucket.Spec.SeedName = ptr.To("seed")
				metav1.SetMetaDataAnnotation(&expectedBackupBucket.ObjectMeta, "gardener.cloud/operation", "reconcile")
				metav1.SetMetaDataAnnotation(&expectedBackupBucket.ObjectMeta, "gardener.cloud/timestamp", fakeClock.Now().UTC().Format(time.RFC3339Nano))
				Expect(actual).To(DeepEqual(expectedBackupBucket))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should not return error when it's not found", func() {
			Expect(deployer.Destroy(ctx)).To(Succeed())
		})

		It("should not return error when it's deleted successfully", func() {
			Expect(fakeClient.Create(ctx, expectedBackupBucket)).To(Succeed())
			Expect(deployer.Destroy(ctx)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedBackupBucket), expectedBackupBucket)).To(BeNotFoundError())
		})
	})

	Describe("#Wait", func() {
		It("should return error when it's not found", func() {
			Expect(deployer.Wait(ctx)).To(HaveOccurred())
		})

		It("should return error when it's not ready", func() {
			expectedBackupBucket.Status.LastError = &gardencorev1beta1.LastError{Description: "fake"}

			Expect(fakeClient.Create(ctx, expectedBackupBucket)).To(Succeed())
			Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("error during reconciliation: fake")))
		})

		It("should return no error when is ready", func() {
			expectedBackupBucket.Status.LastError = nil
			expectedBackupBucket.Annotations = map[string]string{}
			expectedBackupBucket.Status.LastOperation = &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded}

			Expect(fakeClient.Create(ctx, expectedBackupBucket)).To(Succeed())
			Expect(deployer.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error when it's already removed", func() {
			Expect(deployer.WaitCleanup(ctx)).To(Succeed())
		})

		It("should return error when it's not deleted successfully", func() {
			Expect(fakeClient.Create(ctx, expectedBackupBucket)).To(Succeed())
			Expect(deployer.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
		})
	})

	Describe("#Get", func() {
		It("should return error if the BackupBucket does not exist", func() {
			_, err := deployer.Get(ctx)
			Expect(err).To(BeNotFoundError())
		})

		It("should return the retrieved BackupBucket and save it locally", func() {
			Expect(fakeClient.Create(ctx, expectedBackupBucket)).To(Succeed())

			backupBucket, err := deployer.Get(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(backupBucket).To(Equal(expectedBackupBucket))
		})
	})
})
