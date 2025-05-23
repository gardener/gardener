// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/controller/backupbucket"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("BackupBucket controller tests", func() {
	var (
		gardenSecret                      *corev1.Secret
		backupBucket                      *gardencorev1beta1.BackupBucket
		seedGeneratedSecret               *corev1.Secret
		gardenGeneratedSecret             *corev1.Secret
		extensionSecret                   *corev1.Secret
		extensionBackupBucket             *extensionsv1alpha1.BackupBucket
		expectedExtensionBackupBucketSpec extensionsv1alpha1.BackupBucketSpec
		providerStatus                    = &runtime.RawExtension{Raw: []byte(`{"foo":"bar"}`)}
		resourceName                      string

		reconcileExtensionBackupBucket func(bool)
	)

	BeforeEach(func() {
		DeferCleanup(test.WithVar(&backupbucket.RequeueDurationWhenResourceDeletionStillPresent, 30*time.Millisecond))

		fakeClock.SetTime(time.Now().Truncate(time.Second))

		reconcileExtensionBackupBucket = func(makeReady bool) {
			// These should be done by the extension controller, we are faking it here for the tests.
			EventuallyWithOffset(1, func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionSecret), extensionSecret)).To(Succeed())
				g.Expect(extensionSecret.Data).To(Equal(gardenSecret.Data))

				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket)).To(Succeed())
				g.Expect(extensionBackupBucket.Spec).To(Equal(expectedExtensionBackupBucketSpec))
				g.Expect(extensionBackupBucket.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile))
			}).Should(Succeed())

			By("Create generated secret in seed")
			seedGeneratedSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      generateGeneratedBackupBucketSecretName(resourceName),
					Namespace: seedGardenNamespace.Name,
					Labels:    map[string]string{testID: testRunID},
				},
				Data: map[string][]byte{
					"baz": []byte("dash"),
				},
			}

			ExpectWithOffset(1, testClient.Create(ctx, seedGeneratedSecret)).To(Succeed())
			log.Info("Created generated Secret in the Seed for test", "secret", client.ObjectKeyFromObject(seedGeneratedSecret))

			DeferCleanup(func() {
				By("Delete generated Secret in the Seed")
				ExpectWithOffset(1, testClient.Delete(ctx, seedGeneratedSecret)).To(Succeed())
			})

			patch := client.MergeFrom(extensionBackupBucket.DeepCopy())
			delete(extensionBackupBucket.Annotations, v1beta1constants.GardenerOperation)
			ExpectWithOffset(1, testClient.Patch(ctx, extensionBackupBucket, patch)).To(Succeed())

			patch = client.MergeFrom(extensionBackupBucket.DeepCopy())
			lastOperationState := gardencorev1beta1.LastOperationStateSucceeded
			if !makeReady {
				lastOperationState = gardencorev1beta1.LastOperationStateError
			}
			extensionBackupBucket.Status = extensionsv1alpha1.BackupBucketStatus{
				DefaultStatus: extensionsv1alpha1.DefaultStatus{
					ObservedGeneration: extensionBackupBucket.Generation,
					ProviderStatus:     providerStatus,
					LastOperation: &gardencorev1beta1.LastOperation{
						State:          lastOperationState,
						LastUpdateTime: metav1.NewTime(fakeClock.Now()),
					},
				},
				GeneratedSecretRef: &corev1.SecretReference{
					Name:      seedGeneratedSecret.Name,
					Namespace: seedGeneratedSecret.Namespace,
				},
			}
			ExpectWithOffset(1, testClient.Status().Patch(ctx, extensionBackupBucket, patch)).To(Succeed())
		}

		resourceName = "bucket-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:12]

		By("Create BackupBucket secret in garden")
		gardenSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-secret-",
				Namespace:    gardenNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}

		Expect(testClient.Create(ctx, gardenSecret)).To(Succeed())
		log.Info("Created Secret for BackupBucket in garden for test", "secret", client.ObjectKeyFromObject(gardenSecret))

		DeferCleanup(func() {
			By("Delete secret for BackupBucket in garden")
			Expect(testClient.Delete(ctx, gardenSecret)).To(Succeed())
		})

		By("Create BackupBucket")
		backupBucket = &gardencorev1beta1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name:        resourceName,
				Labels:      map[string]string{testID: testRunID},
				Annotations: map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile},
			},
			Spec: gardencorev1beta1.BackupBucketSpec{
				Provider: gardencorev1beta1.BackupBucketProvider{
					Type:   "provider-type",
					Region: "some-region",
				},
				ProviderConfig: &runtime.RawExtension{Raw: []byte(`{"dash":"baz"}`)},
				CredentialsRef: &corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  gardenSecret.Namespace,
					Name:       gardenSecret.Name,
				},
				SeedName: &seed.Name,
			},
		}

		Expect(testClient.Create(ctx, backupBucket)).To(Succeed())
		log.Info("Created BackupBucket for test", "backupBucket", client.ObjectKeyFromObject(backupBucket))

		DeferCleanup(func() {
			By("Delete BackupBucket")
			Expect(testClient.Delete(ctx, backupBucket)).To(Or(Succeed(), BeNotFoundError()))

			By("Ensure BackupBucket is gone")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket)
			}).Should(BeNotFoundError())
		})

		extensionSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      generateBackupBucketSecretName(resourceName),
				Namespace: gardenNamespace.Name,
			},
		}

		extensionBackupBucket = &extensionsv1alpha1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: resourceName,
			},
		}

		expectedExtensionBackupBucketSpec = extensionsv1alpha1.BackupBucketSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           backupBucket.Spec.Provider.Type,
				ProviderConfig: backupBucket.Spec.ProviderConfig,
			},
			Region: backupBucket.Spec.Provider.Region,
			SecretRef: corev1.SecretReference{
				Name:      extensionSecret.Name,
				Namespace: extensionSecret.Namespace,
			},
		}

		gardenGeneratedSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      generateGeneratedBackupBucketSecretName(resourceName),
				Namespace: gardenNamespace.Name,
			},
		}
	})

	Context("reconcile", func() {
		JustBeforeEach(func() {
			By("Ensure finalizer got added and operation annotation is removed")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket)).To(Succeed())
				g.Expect(backupBucket.Finalizers).To(ConsistOf("gardener"))
				g.Expect(backupBucket.Annotations).NotTo(HaveKey("gardener.cloud/operation"))

				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(gardenSecret), gardenSecret)).To(Succeed())
				g.Expect(gardenSecret.Finalizers).To(ConsistOf("gardener.cloud/gardener"))
			}).Should(Succeed())
		})

		Context("when extensions BackupBucket has been reconciled", func() {
			It("should set the BackupBucket status as Succeeded if the extension BackupBucket is ready", func() {
				By("Mimic extension controller and setting extensions BackupBucket to be successfully reconciled")
				reconcileExtensionBackupBucket(true)

				By("Ensure the generated secret is copied to garden")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(gardenGeneratedSecret), gardenGeneratedSecret)).To(Succeed())
					expectedOwnerRef := *metav1.NewControllerRef(backupBucket, gardencorev1beta1.SchemeGroupVersion.WithKind("BackupBucket"))
					g.Expect(gardenGeneratedSecret.OwnerReferences).To(ContainElement(expectedOwnerRef))
					g.Expect(gardenGeneratedSecret.Finalizers).To(ContainElement("core.gardener.cloud/backupbucket"))
				}).Should(Succeed())

				By("Ensure the BackupBucket status is set")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket)).To(Succeed())
					gardenGeneratedSecretRef := &corev1.SecretReference{
						Name:      gardenGeneratedSecret.Name,
						Namespace: gardenGeneratedSecret.Namespace,
					}
					g.Expect(backupBucket.Status.LastError).To(BeNil())
					g.Expect(backupBucket.Status.ProviderStatus).To(Equal(providerStatus))
					g.Expect(backupBucket.Status.GeneratedSecretRef).To(Equal(gardenGeneratedSecretRef))
					g.Expect(backupBucket.Status.ObservedGeneration).To(Equal(backupBucket.Generation))
					g.Expect(backupBucket.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(backupBucket.Status.LastOperation.Progress).To(Equal(int32(100)))
				}).Should(Succeed())
			})

			It("should set the BackupBucket status as Error if the extension BackupBucket is not ready", func() {
				By("Mimic extension controller and making extensions BackupBucket to be reconciled with error")
				reconcileExtensionBackupBucket(false)

				By("Ensure the BackupBucket status is set")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket)).To(Succeed())
					g.Expect(backupBucket.Status.LastError).NotTo(BeNil())
					g.Expect(backupBucket.Status.LastError.Description).To(ContainSubstring("extension state is not Succeeded but Error"))
					g.Expect(backupBucket.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateError))
					g.Expect(backupBucket.Status.LastOperation.Progress).To(Equal(int32(50)))
				}).Should(Succeed())
			})
		})
	})

	Context("delete", func() {
		It("should remove the finalizer and cleanup the resources when the BackupBucket is deleted", func() {
			By("Mimic extension controller and make extensions BackupBucket look successfully reconciled")
			reconcileExtensionBackupBucket(true)

			Expect(testClient.Delete(ctx, backupBucket)).To(Succeed())

			By("Ensure the extension resources are cleaned up")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(gardenGeneratedSecret), gardenGeneratedSecret)).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket)).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionSecret), extensionSecret)).To(BeNotFoundError())
			}).Should(Succeed())

			By("Ensure finalizers are removed and BackupBucket is released")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(gardenSecret), gardenSecret)).To(Succeed())
				g.Expect(gardenSecret.Finalizers).NotTo(ContainElement("gardener.cloud/gardener"))
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket)).To(BeNotFoundError())
			}).Should(Succeed())
		})
	})
})

func generateBackupBucketSecretName(backupBucketName string) string {
	return "bucket-" + backupBucketName
}

func generateGeneratedBackupBucketSecretName(backupBucketName string) string {
	return v1beta1constants.SecretPrefixGeneratedBackupBucket + backupBucketName
}
