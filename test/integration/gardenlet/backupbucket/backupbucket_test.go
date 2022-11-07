// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	backupbucket "github.com/gardener/gardener/pkg/gardenlet/controller/backupbucket"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

		reconcileExtensionBackupBucket = func(makeReady bool) {
			// These should be done by the extension controller, we are faking it here for the tests.
			EventuallyWithOffset(1, func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionSecret), extensionSecret)).To(Succeed())
				g.Expect(extensionSecret.Data).To(Equal(gardenSecret.Data))

				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket)).To(Succeed())
				g.Expect(extensionBackupBucket.Spec).To(Equal(expectedExtensionBackupBucketSpec))
				g.Expect(extensionBackupBucket.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile))
			}).Should(Succeed())

			By("creating generated secret in seed")
			seedGeneratedSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      generateGeneratedBackupBucketSecretName(backupBucket.Name),
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
	)

	BeforeEach(func() {
		By("creating BackupBucket secret in garden")
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
			By("deleting secret for BackupBucket in garden")
			Expect(testClient.Delete(ctx, gardenSecret)).To(Succeed())
		})

		By("creating BackupBucket")
		backupBucket = &gardencorev1beta1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "foo-",
				Labels:       map[string]string{testID: testRunID},
				Annotations:  map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile},
			},
			Spec: gardencorev1beta1.BackupBucketSpec{
				Provider: gardencorev1beta1.BackupBucketProvider{
					Type:   "provider-type",
					Region: "some-region",
				},
				ProviderConfig: &runtime.RawExtension{Raw: []byte(`{"dash":"baz"}`)},
				SecretRef: corev1.SecretReference{
					Name:      gardenSecret.Name,
					Namespace: gardenSecret.Namespace,
				},
				SeedName: &seed.Name,
			},
		}

		Expect(testClient.Create(ctx, backupBucket)).To(Succeed())
		log.Info("Created BackupBucket for test", "backupBucket", client.ObjectKeyFromObject(backupBucket))

		DeferCleanup(func() {
			By("deleting BackupBucket")
			Expect(testClient.Delete(ctx, backupBucket)).To(Or(Succeed(), BeNotFoundError()))
		})

		extensionSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      generateBackupBucketSecretName(backupBucket.Name),
				Namespace: gardenNamespace.Name,
			},
		}

		extensionBackupBucket = &extensionsv1alpha1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: backupBucket.Name,
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
				Name:      generateGeneratedBackupBucketSecretName(backupBucket.Name),
				Namespace: gardenNamespace.Name,
			},
		}
	})

	Context("reconcile", func() {
		JustBeforeEach(func() {
			By("ensuring finalizer got added and operation annotation is removed")
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
				By("Mimicking extension controller and setting extensions BackupBucket to be successfully reconciled")
				reconcileExtensionBackupBucket(true)

				By("ensuring the generated secret is copied to garden")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(gardenGeneratedSecret), gardenGeneratedSecret)).To(Succeed())
					expectedOwnerRef := *metav1.NewControllerRef(backupBucket, gardencorev1beta1.SchemeGroupVersion.WithKind("BackupBucket"))
					g.Expect(gardenGeneratedSecret.OwnerReferences).To(ContainElement(expectedOwnerRef))
					g.Expect(gardenGeneratedSecret.Finalizers).To(ContainElement("core.gardener.cloud/backupbucket"))
				}).Should(Succeed())

				By("ensuring the BackupBucket status is set")
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
				By("Mimicking extension controller and making extensions BackupBucket to be reconciled with error")
				reconcileExtensionBackupBucket(false)

				By("ensuring the BackupBucket status is set")
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
		var backupEntry *gardencorev1beta1.BackupEntry

		BeforeEach(func() {
			DeferCleanup(test.WithVar(&backupbucket.RequeueDurationWhenResourceDeletionStillPresent, 15*time.Millisecond))

			By("creating BackupEntry")
			backupEntry = &gardencorev1beta1.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "backupentry-",
					Namespace:    testNamespace.Name,
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.BackupEntrySpec{
					BucketName: backupBucket.Name,
					SeedName:   pointer.String(seed.Name),
				},
			}

			Expect(testClient.Create(ctx, backupEntry)).To(Succeed())
			log.Info("Created BackupEntry for test", "backupEntry", client.ObjectKeyFromObject(backupEntry))

			By("Wait until mgrClient has observed backupEntry creation")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)
			}).Should(Succeed())

			DeferCleanup(func() {
				By("deleting BackupEntry")
				Expect(testClient.Delete(ctx, backupEntry)).To(Or(Succeed(), BeNotFoundError()))
			})

			By("Mimicking extension controller and make extensions BackupBucket look successfully reconciled")
			reconcileExtensionBackupBucket(true)
		})

		It("should not delete the BackupBucket if there are BackupEntries still referencing it", func() {
			Expect(testClient.Delete(ctx, backupBucket)).To(Succeed())

			By("ensuring BackupBucket is not released")
			Consistently(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket)
			}).Should(Succeed())
		})

		It("should remove the finalizer and cleanup the resources when the BackupBucket is deleted and there are no backupentries referencing it", func() {
			Expect(testClient.Delete(ctx, backupEntry)).To(Succeed())
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)
			}).Should(BeNotFoundError())

			Expect(testClient.Delete(ctx, backupBucket)).To(Succeed())

			By("ensuring the extension resources are cleaned up")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(gardenGeneratedSecret), gardenGeneratedSecret)).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket)).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionSecret), extensionSecret)).To(BeNotFoundError())
			}).Should(Succeed())

			By("ensuring finalizers are removed and BackupBucket is released")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(gardenSecret), gardenSecret)).To(Succeed())
				g.Expect(gardenSecret.Finalizers).NotTo(ContainElement("gardener.cloud/gardener"))
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket)).To(BeNotFoundError())
			}).Should(Succeed())
		})
	})
})

func generateBackupBucketSecretName(backupBucketName string) string {
	return fmt.Sprintf("bucket-%s", backupBucketName)
}

func generateGeneratedBackupBucketSecretName(backupBucketName string) string {
	return v1beta1constants.SecretPrefixGeneratedBackupBucket + backupBucketName
}
