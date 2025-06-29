// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/backupentry"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
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
		workloadIdentity     *securityv1alpha1.WorkloadIdentity
		backupBucket         *gardencorev1beta1.BackupBucket
		backupEntry          *gardencorev1beta1.BackupEntry
		extensionBackupEntry *extensionsv1alpha1.BackupEntry
		extensionSecret      *corev1.Secret
		providerConfig       = &runtime.RawExtension{Raw: []byte(`{"dash":"baz"}`)}
		providerStatus       = &runtime.RawExtension{Raw: []byte(`{"foo":"bar"}`)}
		now                  time.Time

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
		workloadIdentity = &securityv1alpha1.WorkloadIdentity{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-workload-identity",
				Namespace: gardenNamespaceName,
			},
			Spec: securityv1alpha1.WorkloadIdentitySpec{
				Audiences: []string{"test"},
				TargetSystem: securityv1alpha1.TargetSystem{
					Type: "local",
				},
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
				SeedName:       ptr.To(seedName),
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

		now = fakeClock.Now().UTC()
		extensionSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "entry-" + backupEntry.Name,
				Namespace: gardenNamespaceName,
			},
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

	Describe("#Invalid Credentials", func() {
		It("should fail to reconcile with non-existing credentials", func() {
			backupBucket.Spec.CredentialsRef = &corev1.ObjectReference{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Secret",
				Namespace:  "non-existing",
				Name:       "non-existing",
			}
			Expect(gardenClient.Create(ctx, backupBucket)).To(Succeed())
			Expect(gardenClient.Create(ctx, backupEntry)).To(Succeed())
			gardenSecret := corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: backupBucket.Spec.CredentialsRef.Namespace, Name: backupBucket.Spec.CredentialsRef.Name}}
			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(&gardenSecret), &gardenSecret)).To(BeNotFoundError())

			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should fail to reconcile with unsupported credentials", func() {
			backupBucket.Spec.CredentialsRef = &corev1.ObjectReference{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "ConfigMap",
				Namespace:  "garden",
				Name:       "backup-cm",
			}
			Expect(gardenClient.Create(ctx, backupBucket)).To(Succeed())
			Expect(gardenClient.Create(ctx, backupEntry)).To(Succeed())
			gardenConfigMap := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: backupBucket.Spec.CredentialsRef.Namespace, Name: backupBucket.Spec.CredentialsRef.Name}}
			Expect(gardenClient.Create(ctx, &gardenConfigMap)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("could not get credentials referred in core BackupBucket: unsupported credentials reference: garden/backup-cm, /v1, Kind=ConfigMap"))
			Expect(result).To(Equal(reconcile.Result{}))
		})
	})

	Describe("Secrets Credentials", func() {
		BeforeEach(func() {
			backupBucket.Spec.CredentialsRef = &corev1.ObjectReference{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Secret",
				Namespace:  gardenSecret.Namespace,
				Name:       gardenSecret.Name,
			}

			Expect(gardenClient.Create(ctx, gardenSecret)).To(Succeed())
			Expect(gardenClient.Create(ctx, backupBucket)).To(Succeed())
			Expect(gardenClient.Create(ctx, backupEntry)).To(Succeed())
			extensionSecret.Annotations = map[string]string{v1beta1constants.GardenerTimestamp: now.Format(time.RFC3339Nano)}
			extensionSecret.Data = gardenSecret.Data
		})

		It("should create the extension secret and extension BackupEntry if they do not exist yet", func() {
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

		It("should use the backupBucket.status.generatedSecret as credentials", func() {
			generatedGardenSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "generated-secret",
					Namespace: gardenNamespaceName,
				},
				Data: map[string][]byte{
					"generatedSecret1": []byte("generatedValue1"),
					"generatedSecret2": []byte("generatedValue2"),
				},
			}
			backupBucket.Status.GeneratedSecretRef = &corev1.SecretReference{
				Namespace: gardenNamespaceName,
				Name:      "generated-secret",
			}
			Expect(gardenClient.Create(ctx, generatedGardenSecret)).To(Succeed())
			Expect(gardenClient.Status().Update(ctx, backupBucket)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionSecret), extensionSecret)).To(Succeed())
			Expect(extensionSecret.Data).To(Equal(map[string][]byte{
				"generatedSecret1": []byte("generatedValue1"),
				"generatedSecret2": []byte("generatedValue2"),
			}))
		})
	})

	Describe("#WorkloadIdentity Credentials", func() {
		BeforeEach(func() {
			backupBucket.Spec.CredentialsRef = &corev1.ObjectReference{
				APIVersion: securityv1alpha1.SchemeGroupVersion.String(),
				Kind:       "WorkloadIdentity",
				Namespace:  workloadIdentity.Namespace,
				Name:       workloadIdentity.Name,
			}
			backupEntry.TypeMeta = metav1.TypeMeta{APIVersion: "core.gardener.cloud/v1beta1", Kind: "BackupEntry"}
			Expect(gardenClient.Create(ctx, backupBucket)).To(Succeed())
			Expect(gardenClient.Create(ctx, backupEntry)).To(Succeed())
			Expect(gardenClient.Create(ctx, workloadIdentity)).To(Succeed())
			extensionSecret.Annotations = map[string]string{
				"gardener.cloud/timestamp":                                now.Format(time.RFC3339Nano),
				"workloadidentity.security.gardener.cloud/context-object": `{"kind":"BackupEntry","apiVersion":"core.gardener.cloud/v1beta1","name":"bar","namespace":"test","uid":""}`,
				"workloadidentity.security.gardener.cloud/name":           workloadIdentity.Name,
				"workloadidentity.security.gardener.cloud/namespace":      workloadIdentity.Namespace,
			}
			extensionSecret.Labels = map[string]string{
				"security.gardener.cloud/purpose":                   "workload-identity-token-requestor",
				"workloadidentity.security.gardener.cloud/provider": "local",
			}
			extensionSecret.Type = "Opaque"
		})

		It("should create the extension secret and extension BackupEntry if they do not exist yet", func() {
			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionSecret), extensionSecret)).To(Succeed())
			Expect(extensionSecret.Data).To(BeEmpty())
			Expect(extensionSecret.Labels).To(Equal(map[string]string{
				"security.gardener.cloud/purpose":                   "workload-identity-token-requestor",
				"workloadidentity.security.gardener.cloud/provider": "local",
			}))
			Expect(extensionSecret.Annotations).To(HaveKeyWithValue("workloadidentity.security.gardener.cloud/context-object", `{"kind":"BackupEntry","apiVersion":"core.gardener.cloud/v1beta1","name":"bar","namespace":"test","uid":""}`))
			Expect(extensionSecret.Annotations).To(HaveKeyWithValue("workloadidentity.security.gardener.cloud/name", workloadIdentity.Name))
			Expect(extensionSecret.Annotations).To(HaveKeyWithValue("workloadidentity.security.gardener.cloud/namespace", workloadIdentity.Namespace))
			Expect(extensionSecret.Annotations).To(HaveKey("gardener.cloud/timestamp"))
			Expect(extensionSecret.Type).To(BeEquivalentTo("Opaque"))

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
			delete(extensionBackupEntry.Annotations, "gardener.cloud/operation")
			Expect(seedClient.Create(ctx, extensionBackupEntry)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupEntry), extensionBackupEntry)).To(Succeed())
			Expect(extensionBackupEntry.Annotations).NotTo(HaveKey("gardener.cloud/operation"))
		})

		It("should reconcile the extension secret and extension BackupEntry if the secret currently doesn't have a timestamp", func() {
			delete(extensionSecret.Annotations, "gardener.cloud/timestamp")
			Expect(seedClient.Create(ctx, extensionSecret)).To(Succeed())
			Expect(seedClient.Create(ctx, extensionBackupEntry)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionSecret), extensionSecret)).To(Succeed())
			Expect(extensionSecret.Labels).To(Equal(map[string]string{
				"security.gardener.cloud/purpose":                   "workload-identity-token-requestor",
				"workloadidentity.security.gardener.cloud/provider": "local",
			}))
			Expect(extensionSecret.Annotations).To(HaveKeyWithValue("workloadidentity.security.gardener.cloud/context-object", `{"kind":"BackupEntry","apiVersion":"core.gardener.cloud/v1beta1","name":"bar","namespace":"test","uid":""}`))
			Expect(extensionSecret.Annotations).To(HaveKeyWithValue("workloadidentity.security.gardener.cloud/name", workloadIdentity.Name))
			Expect(extensionSecret.Annotations).To(HaveKeyWithValue("workloadidentity.security.gardener.cloud/namespace", workloadIdentity.Namespace))
			Expect(extensionSecret.Annotations).To(HaveKey("gardener.cloud/timestamp"))

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupEntry), extensionBackupEntry)).To(Succeed())
			Expect(extensionBackupEntry.Annotations).To(HaveKeyWithValue("gardener.cloud/operation", "reconcile"))
		})

		It("should reconcile the extension BackupEntry if the secret data has changed", func() {
			// the workloadIdentity.spec.targetSystem.providerConfig=nil will cause the `config` data key to be removed from the secret
			extensionSecret.Data = map[string][]byte{"config": []byte("null")}
			Expect(seedClient.Create(ctx, extensionSecret)).To(Succeed())
			delete(extensionBackupEntry.Annotations, "gardener.cloud/operation")
			Expect(seedClient.Create(ctx, extensionBackupEntry)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupEntry), extensionBackupEntry)).To(Succeed())
			Expect(extensionBackupEntry.Annotations).To(HaveKeyWithValue("gardener.cloud/operation", "reconcile"))
		})
	})
})
