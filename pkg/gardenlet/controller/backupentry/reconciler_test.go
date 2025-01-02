// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/backupentry"
)

const (
	seedName            = "seed"
	gardenNamespaceName = "garden"
	testNamespaceName   = "test"
)

var _ = Describe("Controller", func() {
	var (
		ctx          = context.TODO()
		gardenClient client.Client
		seedClient   client.Client
		reconciler   reconcile.Reconciler

		fakeClock                *testclock.FakeClock
		deletionGracePeriodHours = 24

		gardenSecret         *corev1.Secret
		backupBucket         *gardencorev1beta1.BackupBucket
		backupEntry          *gardencorev1beta1.BackupEntry
		extensionBackupEntry *extensionsv1alpha1.BackupEntry
		extensionSecret      *corev1.Secret
		providerConfig       = &runtime.RawExtension{Raw: []byte(`{"dash":"baz"}`)}
		providerStatus       = &runtime.RawExtension{Raw: []byte(`{"foo":"bar"}`)}

		request reconcile.Request
	)

	BeforeEach(func() {
		gardenSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: gardenNamespaceName,
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}

		backupBucket = &gardencorev1beta1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Generation: 1,
			},
			Spec: gardencorev1beta1.BackupBucketSpec{
				Provider: gardencorev1beta1.BackupBucketProvider{
					Type:   "provider-type",
					Region: "some-region",
				},
				ProviderConfig: providerConfig,
				SecretRef: corev1.SecretReference{
					Name:      gardenSecret.Name,
					Namespace: gardenSecret.Namespace,
				},
				SeedName: ptr.To(seedName),
			},
			Status: gardencorev1beta1.BackupBucketStatus{
				ObservedGeneration: 1,
				LastOperation: &gardencorev1beta1.LastOperation{
					State: gardencorev1beta1.LastOperationStateSucceeded,
					Type:  gardencorev1beta1.LastOperationTypeReconcile,
				},
				ProviderStatus: providerStatus,
			},
		}

		backupEntry = &gardencorev1beta1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bar",
				Namespace: testNamespaceName,
			},
			Spec: gardencorev1beta1.BackupEntrySpec{
				BucketName: backupBucket.Name,
				SeedName:   ptr.To(seedName),
			},
		}

		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithStatusSubresource(&gardencorev1beta1.BackupBucket{}, &gardencorev1beta1.BackupEntry{}).Build()

		testSchemeBuilder := runtime.NewSchemeBuilder(
			kubernetes.AddSeedSchemeToScheme,
			extensionsv1alpha1.AddToScheme,
		)
		testScheme := runtime.NewScheme()
		Expect(testSchemeBuilder.AddToScheme(testScheme)).To(Succeed())

		seedClient = fakeclient.NewClientBuilder().WithScheme(testScheme).Build()

		fakeClock = testclock.NewFakeClock(time.Now())

		reconciler = &Reconciler{
			GardenClient: gardenClient,
			SeedClient:   seedClient,
			Recorder:     &record.FakeRecorder{},
			Config: gardenletconfigv1alpha1.BackupEntryControllerConfiguration{
				ConcurrentSyncs:                  ptr.To(5),
				DeletionGracePeriodHours:         ptr.To(deletionGracePeriodHours),
				DeletionGracePeriodShootPurposes: []gardencorev1beta1.ShootPurpose{gardencorev1beta1.ShootPurposeProduction},
			},
			Clock:           fakeClock,
			GardenNamespace: gardenNamespaceName,
			SeedName:        seedName,
		}

		Expect(gardenClient.Create(ctx, gardenSecret)).To(Succeed())
		Expect(gardenClient.Create(ctx, backupBucket)).To(Succeed())
		Expect(gardenClient.Create(ctx, backupEntry)).To(Succeed())

		now := fakeClock.Now().UTC()
		extensionSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "entry-" + backupEntry.Name,
				Namespace: gardenNamespaceName,
				Annotations: map[string]string{
					v1beta1constants.GardenerTimestamp: now.Format(time.RFC3339Nano),
				},
			},
			Data: gardenSecret.Data,
		}

		extensionBackupEntry = &extensionsv1alpha1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: backupEntry.Name,
			},
			Spec: extensionsv1alpha1.BackupEntrySpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type:           backupBucket.Spec.Provider.Type,
					ProviderConfig: backupBucket.Spec.ProviderConfig,
				},
				Region: backupBucket.Spec.Provider.Region,
				SecretRef: corev1.SecretReference{
					Name:      extensionSecret.Name,
					Namespace: extensionSecret.Namespace,
				},
				BucketName:                 backupEntry.Spec.BucketName,
				BackupBucketProviderStatus: backupBucket.Status.ProviderStatus,
			},
			Status: extensionsv1alpha1.BackupEntryStatus{
				DefaultStatus: extensionsv1alpha1.DefaultStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State:          gardencorev1beta1.LastOperationStateSucceeded,
						LastUpdateTime: metav1.NewTime(now),
					},
				},
			},
		}

		request = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      backupEntry.Name,
				Namespace: backupEntry.Namespace,
			},
		}
	})

	It("should create the extension secret and extension BackupEntry if it doesn't exist yet", func() {
		result, err := reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))

		Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionSecret), extensionSecret)).To(Succeed())
		Expect(extensionSecret.Data).To(Equal(gardenSecret.Data))

		Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupEntry), extensionBackupEntry)).To(Succeed())
		Expect(extensionBackupEntry.Spec).To(MatchFields(IgnoreExtras, Fields{
			"DefaultSpec": MatchFields(IgnoreExtras, Fields{
				"Type":           Equal(backupBucket.Spec.Provider.Type),
				"ProviderConfig": Equal(backupBucket.Spec.ProviderConfig),
			}),
			"Region":                     Equal(backupBucket.Spec.Provider.Region),
			"BackupBucketProviderStatus": Equal(backupBucket.Status.ProviderStatus),
			"SecretRef": MatchFields(IgnoreExtras, Fields{
				"Name":      Equal(extensionSecret.Name),
				"Namespace": Equal(extensionSecret.Namespace),
			}),
		}))
	})

	It("should not reconcile the extension BackupEntry if the secret data or extension spec hasn't changed", func() {
		Expect(seedClient.Create(ctx, extensionSecret)).To(Succeed())
		Expect(seedClient.Create(ctx, extensionBackupEntry)).To(Succeed())

		result, err := reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))

		Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupEntry), extensionBackupEntry)).To(Succeed())
		Expect(extensionBackupEntry.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
	})

	It("should reconcile the extension secret and extension BackupEntry if the secret currently doesn't have a timestamp", func() {
		extensionSecret.Annotations = nil
		Expect(seedClient.Create(ctx, extensionSecret)).To(Succeed())
		Expect(seedClient.Create(ctx, extensionBackupEntry)).To(Succeed())

		// step the clock so that the updated timestamp of the secret is greater than the extensionSecret lastUpdate time.
		fakeClock.Step(time.Minute)

		result, err := reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))

		Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionSecret), extensionSecret)).To(Succeed())
		Expect(extensionSecret.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerTimestamp, fakeClock.Now().UTC().Format(time.RFC3339Nano)))
		Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupEntry), extensionBackupEntry)).To(Succeed())
		Expect(extensionBackupEntry.Annotations).To(HaveKey(v1beta1constants.GardenerOperation))
	})

	It("should reconcile the extension BackupEntry if the secret data has changed", func() {
		extensionSecret.Data = map[string][]byte{"dash": []byte("bash")}
		Expect(seedClient.Create(ctx, extensionSecret)).To(Succeed())
		Expect(seedClient.Create(ctx, extensionBackupEntry)).To(Succeed())

		result, err := reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))

		Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupEntry), extensionBackupEntry)).To(Succeed())
		Expect(extensionBackupEntry.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile))
	})

	It("should reconcile the extension BackupEntry if the secret update timestamp is after the extension last update time", func() {
		time := fakeClock.Now().Add(time.Second).UTC().Format(time.RFC3339Nano)
		metav1.SetMetaDataAnnotation(&extensionSecret.ObjectMeta, v1beta1constants.GardenerTimestamp, time)
		Expect(seedClient.Create(ctx, extensionSecret)).To(Succeed())
		Expect(seedClient.Create(ctx, extensionBackupEntry)).To(Succeed())

		result, err := reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))

		Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupEntry), extensionBackupEntry)).To(Succeed())
		Expect(extensionBackupEntry.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile))
	})
})
