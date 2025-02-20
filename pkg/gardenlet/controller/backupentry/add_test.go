// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/backupentry"
)

var _ = Describe("Add", func() {
	var (
		reconciler       *Reconciler
		backupEntry      *gardencorev1beta1.BackupEntry
		projectName      = "dev"
		shootName        = "shoot"
		projectNamespace string
		shootTechnicalID string
		entryName        string
	)

	BeforeEach(func() {
		shootTechnicalID = fmt.Sprintf("shoot--%s--%s", projectName, shootName)
		entryName = shootTechnicalID + "--shootUID"
		projectNamespace = "garden-" + projectName

		reconciler = &Reconciler{
			SeedName: "seed",
		}

		backupEntry = &gardencorev1beta1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      entryName,
				Namespace: projectNamespace,
			},
			Spec: gardencorev1beta1.BackupEntrySpec{
				SeedName: ptr.To("seed"),
			},
		}
	})

	Describe("#MapExtensionBackupEntryToBackupEntry", func() {
		var (
			ctx                  = context.TODO()
			log                  = logr.Discard()
			extensionBackupEntry *extensionsv1alpha1.BackupEntry
			cluster              *extensionsv1alpha1.Cluster
			fakeClient           client.Client
		)

		BeforeEach(func() {
			extensionBackupEntry = &extensionsv1alpha1.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name: entryName,
				},
			}

			cluster = &extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: shootTechnicalID,
				},
				Spec: extensionsv1alpha1.ClusterSpec{
					Shoot: runtime.RawExtension{
						Object: &gardencorev1beta1.Shoot{
							TypeMeta: metav1.TypeMeta{
								APIVersion: "core.gardener.cloud/v1beta1",
								Kind:       "Shoot",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:      shootName,
								Namespace: projectNamespace,
							},
						},
					},
				},
			}

			testScheme := runtime.NewScheme()
			Expect(extensionsv1alpha1.AddToScheme(testScheme)).To(Succeed())

			fakeClient = fakeclient.NewClientBuilder().WithScheme(testScheme).Build()
			reconciler = &Reconciler{
				SeedName:   "seed",
				SeedClient: fakeClient,
			}
		})

		It("should return a request with the core.gardener.cloud/v1beta1.BackupEntry name and namespace", func() {
			Expect(fakeClient.Create(ctx, cluster)).To(Succeed())
			Expect(reconciler.MapExtensionBackupEntryToCoreBackupEntry(log)(ctx, extensionBackupEntry)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: backupEntry.Name, Namespace: backupEntry.Namespace}},
			))
		})

		It("should return nil if the object has a deletion timestamp", func() {
			extensionBackupEntry.DeletionTimestamp = &metav1.Time{Time: time.Now()}

			Expect(reconciler.MapExtensionBackupEntryToCoreBackupEntry(log)(ctx, extensionBackupEntry)).To(BeNil())
		})

		It("should return nil when cluster is not found", func() {
			Expect(reconciler.MapExtensionBackupEntryToCoreBackupEntry(log)(ctx, extensionBackupEntry)).To(BeNil())
		})

		It("should return nil when shoot is not present in the cluster", func() {
			cluster.Spec.Shoot.Object = nil
			Expect(fakeClient.Create(ctx, cluster)).To(Succeed())

			Expect(reconciler.MapExtensionBackupEntryToCoreBackupEntry(log)(ctx, extensionBackupEntry)).To(BeNil())
		})
	})

	Describe("#MapBackupBucketToBackupEntry", func() {
		var (
			ctx          = context.TODO()
			log          = logr.Discard()
			backupEntry1 *gardencorev1beta1.BackupEntry
			backupEntry2 *gardencorev1beta1.BackupEntry
			backupEntry3 *gardencorev1beta1.BackupEntry
			backupBucket *gardencorev1beta1.BackupBucket
			fakeClient   client.Client
		)

		BeforeEach(func() {
			backupBucket = &gardencorev1beta1.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bucket",
				},
			}

			backupEntry1 = &gardencorev1beta1.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "entry-1",
					Namespace: "garden-test1",
				},
				Spec: gardencorev1beta1.BackupEntrySpec{
					BucketName: backupBucket.Name,
				},
			}

			backupEntry2 = &gardencorev1beta1.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "entry-2",
					Namespace: "garden-test2",
				},
				Spec: gardencorev1beta1.BackupEntrySpec{
					BucketName: "random-bucket",
				},
			}

			backupEntry3 = &gardencorev1beta1.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "entry-3",
					Namespace: "garden-test3",
				},
				Spec: gardencorev1beta1.BackupEntrySpec{
					BucketName: backupBucket.Name,
				},
			}

			testScheme := runtime.NewScheme()
			Expect(kubernetes.AddGardenSchemeToScheme(testScheme)).To(Succeed())

			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(testScheme).
				WithIndex(&gardencorev1beta1.BackupEntry{}, core.BackupEntryBucketName, indexer.BackupEntryBucketNameIndexerFunc).
				Build()
			reconciler = &Reconciler{
				GardenClient: fakeClient,
			}
		})

		It("should return nil when the object is not BackupBucket", func() {
			Expect(reconciler.MapBackupBucketToBackupEntry(log)(ctx, &corev1.Secret{})).To(BeNil())
		})

		It("should return requests with the name and namespace of backupentries referencing this backupbucket", func() {
			Expect(fakeClient.Create(ctx, backupEntry1)).To(Succeed())
			Expect(fakeClient.Create(ctx, backupEntry2)).To(Succeed())
			Expect(fakeClient.Create(ctx, backupEntry3)).To(Succeed())

			Expect(reconciler.MapBackupBucketToBackupEntry(log)(ctx, backupBucket)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: backupEntry1.Name, Namespace: backupEntry1.Namespace}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: backupEntry3.Name, Namespace: backupEntry3.Namespace}},
			))
		})

		Context("when backupentry is being migrated to a different seed", func() {
			It("should return requests only with the name and namespace of backupentries that were migrated successfully and annotated with `restore`", func() {
				backupEntry1.Annotations = map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationRestore,
				}
				backupEntry1.Status.LastOperation = &gardencorev1beta1.LastOperation{
					State: gardencorev1beta1.LastOperationStateSucceeded,
					Type:  gardencorev1beta1.LastOperationTypeMigrate,
				}

				backupEntry3.Status.LastOperation = &gardencorev1beta1.LastOperation{
					State: gardencorev1beta1.LastOperationStateSucceeded,
					Type:  gardencorev1beta1.LastOperationTypeMigrate,
				}

				Expect(fakeClient.Create(ctx, backupEntry1)).To(Succeed())
				Expect(fakeClient.Create(ctx, backupEntry2)).To(Succeed())
				Expect(fakeClient.Create(ctx, backupEntry3)).To(Succeed())

				Expect(reconciler.MapBackupBucketToBackupEntry(log)(ctx, backupBucket)).To(ConsistOf(
					reconcile.Request{NamespacedName: types.NamespacedName{Name: backupEntry1.Name, Namespace: backupEntry1.Namespace}},
				))
			})

			It("should return requests with the name and namespace of backupentry which was not successfully migrated", func() {
				backupEntry1.Status.LastOperation = &gardencorev1beta1.LastOperation{
					State: gardencorev1beta1.LastOperationStateError,
					Type:  gardencorev1beta1.LastOperationTypeMigrate,
				}

				Expect(fakeClient.Create(ctx, backupEntry1)).To(Succeed())

				Expect(reconciler.MapBackupBucketToBackupEntry(log)(ctx, backupBucket)).To(ConsistOf(
					reconcile.Request{NamespacedName: types.NamespacedName{Name: backupEntry1.Name, Namespace: backupEntry1.Namespace}},
				))
			})
		})
	})

	Describe("#BackupEntryPredicate", func() {
		var (
			backupEntry *gardencorev1beta1.BackupEntry
			reconciler  = &Reconciler{SeedName: "seed"}
		)

		BeforeEach(func() {
			backupEntry = &gardencorev1beta1.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      entryName,
					Namespace: projectNamespace,
				},
				Spec: gardencorev1beta1.BackupEntrySpec{
					SeedName: ptr.To("seed"),
				},
			}
		})

		It("should return false when the object is not BackupEntry", func() {
			Expect((reconciler).BackupEntryPredicate(&corev1.Secret{})).To(BeFalse())
		})

		It("should return true when the seed is responsible for the backupentry (spec.seedName match)", func() {
			Expect(reconciler.BackupEntryPredicate(backupEntry)).To(BeTrue())
		})

		It("should return true when the seed is responsible for the backupentry (status.seedName match)", func() {
			backupEntry.Spec.SeedName = ptr.To("another-seed")
			backupEntry.Status.SeedName = ptr.To("seed")
			Expect(reconciler.BackupEntryPredicate(backupEntry)).To(BeTrue())
		})

		It("should return false when the seed is not responsible for the backupentry (spec.seedName doesn't match)", func() {
			backupEntry.Spec.SeedName = ptr.To("another-seed")
			Expect(reconciler.BackupEntryPredicate(backupEntry)).To(BeFalse())
		})

		It("should return false when the seed is not responsible for the backupentry (status.seedName doesn't match)", func() {
			backupEntry.Status.SeedName = ptr.To("another-seed")
			Expect(reconciler.BackupEntryPredicate(backupEntry)).To(BeFalse())
		})

		It("should return false when the backupentry is successfully migrated", func() {
			backupEntry.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(reconciler.BackupEntryPredicate(backupEntry)).To(BeFalse())
		})

		It("should return true when the backupentry is not successfully migrated", func() {
			Expect(reconciler.BackupEntryPredicate(backupEntry)).To(BeTrue())
		})

		It("should return true when the backupentry has `restore` annotation", func() {
			backupEntry.Annotations = map[string]string{
				v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationRestore,
			}

			Expect(reconciler.BackupEntryPredicate(backupEntry)).To(BeTrue())
		})
	})
})
