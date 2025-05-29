// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
	. "github.com/gardener/gardener/pkg/gardenlet/controller/backupbucket"
)

const (
	seedName            = "seed"
	gardenNamespaceName = "garden"
)

var _ = Describe("Controller", func() {
	var (
		ctx          = context.TODO()
		gardenClient client.Client
		seedClient   client.Client
		reconciler   reconcile.Reconciler

		fakeClock *testclock.FakeClock

		gardenSecret          *corev1.Secret
		backupBucket          *gardencorev1beta1.BackupBucket
		extensionBackupBucket *extensionsv1alpha1.BackupBucket
		extensionSecret       *corev1.Secret
		providerConfig        = &runtime.RawExtension{Raw: []byte(`{"dash":"baz"}`)}
		providerStatus        = &runtime.RawExtension{Raw: []byte(`{"foo":"bar"}`)}

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
				CredentialsRef: &corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  gardenSecret.Namespace,
					Name:       gardenSecret.Name,
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

		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithStatusSubresource(&gardencorev1beta1.BackupBucket{}).Build()

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
			Config: gardenletconfigv1alpha1.BackupBucketControllerConfiguration{
				ConcurrentSyncs: ptr.To(5),
			},
			Clock:           fakeClock,
			GardenNamespace: gardenNamespaceName,
			SeedName:        seedName,
		}

		Expect(gardenClient.Create(ctx, gardenSecret)).To(Succeed())
		Expect(gardenClient.Create(ctx, backupBucket)).To(Succeed())

		now := fakeClock.Now().UTC()
		extensionSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      generateBackupBucketSecretName(backupBucket.Name),
				Namespace: gardenNamespaceName,
				Annotations: map[string]string{
					v1beta1constants.GardenerTimestamp: now.Format(time.RFC3339Nano),
				},
			},
			Data: gardenSecret.Data,
		}

		extensionBackupBucket = &extensionsv1alpha1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: backupBucket.Name,
			},
			Spec: extensionsv1alpha1.BackupBucketSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type:           backupBucket.Spec.Provider.Type,
					ProviderConfig: backupBucket.Spec.ProviderConfig,
				},
				Region: backupBucket.Spec.Provider.Region,
				SecretRef: corev1.SecretReference{
					Name:      extensionSecret.Name,
					Namespace: extensionSecret.Namespace,
				},
			},
			Status: extensionsv1alpha1.BackupBucketStatus{
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
				Name:      backupBucket.Name,
				Namespace: backupBucket.Namespace,
			},
		}
	})

	It("should create the extension secret and extension BackupBucket if it doesn't exist yet", func() {
		result, err := reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))

		Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionSecret), extensionSecret)).To(Succeed())
		Expect(extensionSecret.Data).To(Equal(gardenSecret.Data))

		Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket)).To(Succeed())
		Expect(extensionBackupBucket.Spec).To(Equal(extensionsv1alpha1.BackupBucketSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           backupBucket.Spec.Provider.Type,
				ProviderConfig: backupBucket.Spec.ProviderConfig,
			},
			Region: backupBucket.Spec.Provider.Region,
			SecretRef: corev1.SecretReference{
				Name:      extensionSecret.Name,
				Namespace: extensionSecret.Namespace,
			},
		}))
	})

	It("should not reconcile the extension BackupBucket if the secret data or extension spec hasn't changed", func() {
		Expect(seedClient.Create(ctx, extensionSecret)).To(Succeed())
		Expect(seedClient.Create(ctx, extensionBackupBucket)).To(Succeed())

		result, err := reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))

		Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket)).To(Succeed())
		Expect(extensionBackupBucket.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
	})

	It("should reconcile the extension secret and extension BackupBucket if the secret currently doesn't have a timestamp", func() {
		extensionSecret.Annotations = nil
		Expect(seedClient.Create(ctx, extensionSecret)).To(Succeed())
		Expect(seedClient.Create(ctx, extensionBackupBucket)).To(Succeed())

		// step the clock so that the updated timestamp of the secret is greater than the extensionSecret lastUpdate time.
		fakeClock.Step(time.Minute)

		result, err := reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))

		Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionSecret), extensionSecret)).To(Succeed())
		Expect(extensionSecret.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerTimestamp, fakeClock.Now().UTC().Format(time.RFC3339Nano)))
		Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket)).To(Succeed())
		Expect(extensionBackupBucket.Annotations).To(HaveKey(v1beta1constants.GardenerOperation))
	})

	It("should reconcile the extension BackupBucket if the secret data has changed", func() {
		extensionSecret.Data = map[string][]byte{"dash": []byte("bash")}
		Expect(seedClient.Create(ctx, extensionSecret)).To(Succeed())
		Expect(seedClient.Create(ctx, extensionBackupBucket)).To(Succeed())

		result, err := reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))

		Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket)).To(Succeed())
		Expect(extensionBackupBucket.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile))
	})

	It("should reconcile the extension BackupBucket if the secret update timestamp is after the extension last update time", func() {
		time := fakeClock.Now().Add(time.Second).UTC().Format(time.RFC3339Nano)
		metav1.SetMetaDataAnnotation(&extensionSecret.ObjectMeta, v1beta1constants.GardenerTimestamp, time)
		Expect(seedClient.Create(ctx, extensionSecret)).To(Succeed())
		Expect(seedClient.Create(ctx, extensionBackupBucket)).To(Succeed())

		result, err := reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))

		Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket)).To(Succeed())
		Expect(extensionBackupBucket.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile))
	})

	It("should reconcile the extension BackupBucket if the extension's generatedSecretRef has renew-key annotation and the time has already passed", func() {
		generatedSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "generated-secret-" + backupBucket.Name,
				Namespace: gardenNamespaceName,
				Annotations: map[string]string{
					"backupbucket.gardener.cloud/renew-key-timestamp": fakeClock.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano),
				},
			},
		}

		Expect(seedClient.Create(ctx, generatedSecret)).To(Succeed())
		Expect(seedClient.Create(ctx, extensionSecret)).To(Succeed())

		extensionBackupBucket.Status.GeneratedSecretRef = &corev1.SecretReference{
			Name:      generatedSecret.Name,
			Namespace: generatedSecret.Namespace,
		}
		Expect(seedClient.Create(ctx, extensionBackupBucket)).To(Succeed())

		result, err := reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))

		Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket)).To(Succeed())
		Expect(extensionBackupBucket.Annotations).To(HaveKey(v1beta1constants.GardenerOperation))
	})

	It("should not reconcile the extension BackupBucket if the extension's generatedSecretRef has no renew-key annotation", func() {
		generatedSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "generated-secret-" + backupBucket.Name,
				Namespace: gardenNamespaceName,
			},
		}

		Expect(seedClient.Create(ctx, generatedSecret)).To(Succeed())
		Expect(seedClient.Create(ctx, extensionSecret)).To(Succeed())

		extensionBackupBucket.Status.GeneratedSecretRef = &corev1.SecretReference{
			Name:      generatedSecret.Name,
			Namespace: generatedSecret.Namespace,
		}
		Expect(seedClient.Create(ctx, extensionBackupBucket)).To(Succeed())

		result, err := reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))

		Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket)).To(Succeed())
		Expect(extensionBackupBucket.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
	})

	It("should not reconcile the extension BackupBucket if the extension's generatedSecretRef has renew-key annotation but the time has not passed", func() {
		generatedSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "generated-secret-" + backupBucket.Name,
				Namespace: gardenNamespaceName,
				Annotations: map[string]string{
					"backupbucket.gardener.cloud/renew-key-timestamp": fakeClock.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano),
				},
			},
		}

		Expect(seedClient.Create(ctx, generatedSecret)).To(Succeed())
		Expect(seedClient.Create(ctx, extensionSecret)).To(Succeed())

		extensionBackupBucket.Status.GeneratedSecretRef = &corev1.SecretReference{
			Name:      generatedSecret.Name,
			Namespace: generatedSecret.Namespace,
		}
		Expect(seedClient.Create(ctx, extensionBackupBucket)).To(Succeed())

		result, err := reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))

		Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket)).To(Succeed())
		Expect(extensionBackupBucket.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
	})
})

func generateBackupBucketSecretName(backupBucketName string) string {
	return "bucket-" + backupBucketName
}
