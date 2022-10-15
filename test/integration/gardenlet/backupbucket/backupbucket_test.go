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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("BackupBucket controller tests", func() {
	var (
		backupBucket                      *gardencorev1beta1.BackupBucket
		generatedSecret                   *corev1.Secret
		coreGeneratedSecret               *corev1.Secret
		extensionSecret                   *corev1.Secret
		generatedSecretRef                *corev1.SecretReference
		coreGeneratedSecretRef            *corev1.SecretReference
		extensionBackupBucket             *extensionsv1alpha1.BackupBucket
		expectedExtensionBackupBucketSpec extensionsv1alpha1.BackupBucketSpec

		backupBucketReady = func(makeReady bool) {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionSecret), extensionSecret)).To(Succeed())
				g.Expect(extensionSecret.Data).To(Equal(secret.Data))

				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket)).To(Succeed())
				g.Expect(extensionBackupBucket.Spec).To(Equal(expectedExtensionBackupBucketSpec))
				g.Expect(extensionBackupBucket.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile))
			}).Should(Succeed())

			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket)).To(Succeed())
			patch := client.MergeFrom(extensionBackupBucket.DeepCopy())
			delete(extensionBackupBucket.Annotations, v1beta1constants.GardenerOperation)
			Expect(testClient.Patch(ctx, extensionBackupBucket, patch)).To(Succeed())

			// These should be done by the extension controller, we are faking it here for the tests.
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket)).To(Succeed())
			patch = client.MergeFrom(extensionBackupBucket.DeepCopy())

			lastOperationState := gardencorev1beta1.LastOperationStateSucceeded
			if !makeReady {
				lastOperationState = gardencorev1beta1.LastOperationStateError
			}
			extensionBackupBucket.Status = extensionsv1alpha1.BackupBucketStatus{
				DefaultStatus: extensionsv1alpha1.DefaultStatus{
					ObservedGeneration: extensionBackupBucket.Generation,
					ProviderStatus:     &runtime.RawExtension{Raw: []byte("{\"foo\":\"bar\"}")},
					LastOperation: &gardencorev1beta1.LastOperation{
						State:          lastOperationState,
						LastUpdateTime: metav1.NewTime(fakeClock.Now()),
					},
				},
				GeneratedSecretRef: generatedSecretRef,
			}
			Expect(testClient.Status().Patch(ctx, extensionBackupBucket, patch)).To(Succeed())
		}
	)

	BeforeEach(func() {
		By("Create BackupBucket")
		backupBucket = &gardencorev1beta1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "foo-",
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.BackupBucketSpec{
				Provider: gardencorev1beta1.BackupBucketProvider{
					Type:   "provider-type",
					Region: "some-region",
				},
				ProviderConfig: &runtime.RawExtension{Raw: []byte("{\"dash\":\"baz\"}")},
				SecretRef: corev1.SecretReference{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
				SeedName: &seed.Name,
			},
		}

		Expect(testClient.Create(ctx, backupBucket)).To(Succeed())
		log.Info("Created backupBucket for test", "backupBucket", client.ObjectKeyFromObject(backupBucket))

		DeferCleanup(func() {
			By("Delete BackupBucket")
			Expect(testClient.Delete(ctx, backupBucket)).To(Or(Succeed(), BeNotFoundError()))
		})

		extensionSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      generateBackupBucketSecretName(backupBucket.Name),
				Namespace: v1beta1constants.GardenNamespace,
			},
		}

		extensionBackupBucket = &extensionsv1alpha1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: backupBucket.Name,
			},
		}

		generatedSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      generateGeneratedBackupBucketSecretName(backupBucket.Name),
				Namespace: seedGardenNamespace.Name,
				Labels:    map[string]string{testID: testRunID},
			},
			Data: map[string][]byte{
				"baz": []byte("dash"),
			},
		}

		generatedSecretRef = &corev1.SecretReference{
			Name:      generatedSecret.Name,
			Namespace: generatedSecret.Namespace,
		}

		coreGeneratedSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      generateGeneratedBackupBucketSecretName(backupBucket.Name),
				Namespace: gardenNamespace.Name,
			},
		}

		coreGeneratedSecretRef = &corev1.SecretReference{
			Name:      coreGeneratedSecret.Name,
			Namespace: coreGeneratedSecret.Namespace,
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

		Expect(testClient.Create(ctx, generatedSecret)).To(Succeed())
		log.Info("Created secret for test", "secret", client.ObjectKeyFromObject(generatedSecret))

		DeferCleanup(func() {
			By("Delete Secret")
			Expect(testClient.Delete(ctx, generatedSecret)).To(Succeed())
		})
	})

	Context("reconcile", func() {
		BeforeEach(func() {
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
		})

		It("should add the finalizer to the BackupBucket and the referred secret", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket)).To(Succeed())
				g.Expect(backupBucket.Finalizers).To(ConsistOf("gardener"))

				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				g.Expect(secret.Finalizers).To(ConsistOf("gardener.cloud/gardener"))
			}).Should(Succeed())
		})

		Context("set status of the BackupBucket after reconcilation of the extension BackupBucket", func() {
			It("should set the BackupBucket status as Succeeded if the extension BackupBucket is ready", func() {
				backupBucketReady(true)

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(coreGeneratedSecret), coreGeneratedSecret)).To(Succeed())
					expectedOwnerRef := *metav1.NewControllerRef(backupBucket, gardencorev1beta1.SchemeGroupVersion.WithKind("BackupBucket"))
					g.Expect(coreGeneratedSecret.OwnerReferences).To(ContainElement(expectedOwnerRef))
					g.Expect(coreGeneratedSecret.Finalizers).To(ContainElement("core.gardener.cloud/backupbucket"))

					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket)).To(Succeed())
					g.Expect(backupBucket.Status.GeneratedSecretRef).To(Equal(coreGeneratedSecretRef))
					g.Expect(backupBucket.Status.ProviderStatus).NotTo(BeNil())
					g.Expect(backupBucket.Status.ProviderStatus.Raw).To(Equal([]byte("{\"foo\":\"bar\"}")))

					g.Expect(backupBucket.Status.LastError).To(BeNil())
					g.Expect(backupBucket.Status.LastOperation).To(PointTo(MatchFields(IgnoreExtras, Fields{
						"State":    Equal(gardencorev1beta1.LastOperationStateSucceeded),
						"Progress": Equal(int32(100)),
					})))
					g.Expect(backupBucket.Status.ObservedGeneration).To(Equal(backupBucket.Generation))
				}).Should(Succeed())
			})
		})
	})

	Context("delete", func() {
		var backupEntry *gardencorev1beta1.BackupEntry

		BeforeEach(func() {
			backupEntry = &gardencorev1beta1.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "backupentry-",
					Namespace:    testNamespace.Name,
				},
				Spec: gardencorev1beta1.BackupEntrySpec{
					BucketName: backupBucket.Name,
					SeedName:   pointer.String(seed.Name),
				},
			}

			Expect(testClient.Create(ctx, backupEntry)).To(Succeed())
			log.Info("Created backupEntry for test", "backupEntry", client.ObjectKeyFromObject(backupEntry))

			DeferCleanup(func() {
				By("Delete BackupEntry")
				Expect(testClient.Delete(ctx, backupEntry)).To(Or(Succeed(), BeNotFoundError()))
			})

			backupBucketReady(true)
		})

		It("should not delete the BackupBucket if there are backupEntries still referencing it", func() {
			Expect(testClient.Delete(ctx, backupBucket)).To(Succeed())

			Consistently(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket)
			}).Should(Succeed())
		})

		It("should remove the finalizer and cleanup the resources when the BackupBucket is deleted and there are no backupentries referencing it", func() {
			Expect(testClient.Delete(ctx, backupEntry)).To(Succeed())
			Expect(testClient.Delete(ctx, backupBucket)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(coreGeneratedSecret), coreGeneratedSecret)).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket)).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionSecret), extensionSecret)).To(BeNotFoundError())
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				g.Expect(secret.Finalizers).NotTo(ContainElement("gardener.cloud/gardener"))
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
