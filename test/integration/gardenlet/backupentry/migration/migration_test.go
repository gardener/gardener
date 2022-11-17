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

package backupentry_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("BackupEntry migration controller tests", func() {
	var (
		gardenSecret   *corev1.Secret
		providerConfig = &runtime.RawExtension{Raw: []byte(`{"dash":"baz"}`)}
		backupBucket   *gardencorev1beta1.BackupBucket
		backupEntry    *gardencorev1beta1.BackupEntry
		sourceSeed     *gardencorev1beta1.Seed
	)

	BeforeEach(func() {
		fakeClock.SetTime(time.Now().Round(time.Second))

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
				Generation:   1,
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
				SeedName: &seed.Name,
			},
		}

		Expect(testClient.Create(ctx, backupBucket)).To(Succeed())
		log.Info("Created BackupBucket for test", "backupBucket", client.ObjectKeyFromObject(backupBucket))

		By("Ensure manager cache observes BackupBucket creation")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket)
		}).Should(Succeed())

		DeferCleanup(func() {
			By("deleting BackupBucket")
			Expect(testClient.Delete(ctx, backupBucket)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("creating source seed")
		sourceSeed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "seed-",
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.SeedSpec{
				Provider: gardencorev1beta1.SeedProvider{
					Region: "region",
					Type:   "providerType",
				},
				Networks: gardencorev1beta1.SeedNetworks{
					Pods:     "10.0.0.0/16",
					Services: "10.1.0.0/16",
					Nodes:    pointer.String("10.2.0.0/16"),
				},
				DNS: gardencorev1beta1.SeedDNS{
					IngressDomain: pointer.String("someotheringress.example.com"),
				},
				Settings: &gardencorev1beta1.SeedSettings{
					OwnerChecks: &gardencorev1beta1.SeedSettingOwnerChecks{
						Enabled: true,
					},
				},
			},
		}
		Expect(testClient.Create(ctx, sourceSeed)).To(Succeed())
		log.Info("Created source Seed for migration", "seed", sourceSeed.Name)

		DeferCleanup(func() {
			By("deleting source seed")
			Expect(testClient.Delete(ctx, sourceSeed)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("creating BackupEntry for migration")
		backupEntry = &gardencorev1beta1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "backupentry-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
				Annotations: map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
					v1beta1constants.ShootPurpose:      string(gardencorev1beta1.ShootPurposeProduction),
				},
			},
			Spec: gardencorev1beta1.BackupEntrySpec{
				BucketName: backupBucket.Name,
				SeedName:   pointer.String(sourceSeed.Name),
			},
		}

		Expect(testClient.Create(ctx, backupEntry)).To(Succeed())
		log.Info("Created BackupEntry for test", "backupEntry", client.ObjectKeyFromObject(backupEntry))

		DeferCleanup(func() {
			By("deleting BackupEntry")
			Expect(testClient.Delete(ctx, backupEntry)).To(Or(Succeed(), BeNotFoundError()))
		})

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)).To(Succeed())
		patch := client.MergeFrom(backupEntry.DeepCopy())
		backupEntry.Status.SeedName = pointer.String(sourceSeed.Name)
		Expect(testClient.Status().Patch(ctx, backupEntry, patch)).To(Succeed())

		patch = client.MergeFrom(backupEntry.DeepCopy())
		backupEntry.Spec.SeedName = pointer.String(seed.Name)
		Expect(testClient.Patch(ctx, backupEntry, patch)).To(Succeed())
	})

	It("should set migration start time", func() {
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)).To(Succeed())
			g.Expect(backupEntry.Status.MigrationStartTime).To(PointTo(Equal(metav1.Time{Time: fakeClock.Now()})))
		}).Should(Succeed())
	})

	It("should update the backup entry status to force the restoration if forceRestore annotation is present", func() {
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)).To(Succeed())
		patch := client.MergeFrom(backupEntry.DeepCopy())
		metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.AnnotationShootForceRestore, "true")
		Expect(testClient.Patch(ctx, backupEntry, patch)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)).To(Succeed())
			g.Expect(backupEntry.Status.MigrationStartTime).To(BeNil())
			g.Expect(backupEntry.Status.SeedName).To(BeNil())
			g.Expect(backupEntry.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeMigrate))
			g.Expect(backupEntry.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateAborted))
			g.Expect(backupEntry.Annotations).NotTo(HaveKey(v1beta1constants.AnnotationShootForceRestore))
		}).Should(Succeed())
	})

	It("should update the backup entry status to force the restoration if grace period is elapsed", func() {
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)).To(Succeed())
			g.Expect(backupEntry.Status.MigrationStartTime).To(PointTo(Equal(metav1.Time{Time: fakeClock.Now()})))
		}).Should(Succeed())

		fakeClock.Step(2 * gracePeriod)

		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)).To(Succeed())
			g.Expect(backupEntry.Status.MigrationStartTime).To(BeNil())
			g.Expect(backupEntry.Status.SeedName).To(BeNil())
			g.Expect(backupEntry.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeMigrate))
			g.Expect(backupEntry.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateAborted))
		}).Should(Succeed())
	})
})
