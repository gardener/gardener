// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/controller/backupentry"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("BackupEntry controller tests", func() {
	var (
		gardenSecret         *corev1.Secret
		backupBucket         *gardencorev1beta1.BackupBucket
		backupEntry          *gardencorev1beta1.BackupEntry
		extensionBackupEntry *extensionsv1alpha1.BackupEntry
		extensionSecret      *corev1.Secret
		providerConfig       = &runtime.RawExtension{Raw: []byte(`{"dash":"baz"}`)}
		providerStatus       = &runtime.RawExtension{Raw: []byte(`{"foo":"bar"}`)}
		annotations          map[string]string
		shootPurpose         = string(gardencorev1beta1.ShootPurposeProduction)
		shootTechnicalID     string
		shootState           *gardencorev1beta1.ShootState
		shoot                *gardencorev1beta1.Shoot
		shootNamespace       *corev1.Namespace
		cluster              *extensionsv1alpha1.Cluster

		reconcileExtensionBackupEntry func(bool)
		reconcileCoreBackupBucket     func(bool)
	)

	JustBeforeEach(func() {
		DeferCleanup(test.WithVars(
			&backupentry.DefaultTimeout, 1000*time.Millisecond,
			&backupentry.DefaultInterval, 10*time.Millisecond,
			&backupentry.DefaultSevereThreshold, 600*time.Millisecond,

			&backupentry.RequeueDurationWhenResourceDeletionStillPresent, 15*time.Millisecond,
		))

		fakeClock.SetTime(time.Now().Round(time.Second))

		reconcileExtensionBackupEntry = func(makeReady bool) {
			By("Ensure extension secret and extension backupentry is created")
			EventuallyWithOffset(1, func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionSecret), extensionSecret)).To(Succeed())
				g.Expect(extensionSecret.Data).To(Equal(gardenSecret.Data))

				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupEntry), extensionBackupEntry)).To(Succeed())
				g.Expect(extensionBackupEntry.Annotations).To(HaveKey("gardener.cloud/operation"))
				g.Expect(extensionBackupEntry.Spec).To(MatchFields(IgnoreExtras, Fields{
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
			}).Should(Succeed())

			// These should be done by the extension controller, we are faking it here for the tests.
			ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupEntry), extensionBackupEntry)).To(Succeed())
			operationType := extensionBackupEntry.Annotations[v1beta1constants.GardenerOperation]
			patch := client.MergeFrom(extensionBackupEntry.DeepCopy())
			if operationType == v1beta1constants.GardenerOperationReconcile {
				delete(extensionBackupEntry.Annotations, v1beta1constants.GardenerOperation)
				ExpectWithOffset(1, testClient.Patch(ctx, extensionBackupEntry, patch)).To(Succeed())
			}

			patch = client.MergeFrom(extensionBackupEntry.DeepCopy())
			lastOperationState := gardencorev1beta1.LastOperationStateSucceeded
			if !makeReady {
				lastOperationState = gardencorev1beta1.LastOperationStateError
			}
			extensionBackupEntry.Status = extensionsv1alpha1.BackupEntryStatus{
				DefaultStatus: extensionsv1alpha1.DefaultStatus{
					ObservedGeneration: extensionBackupEntry.Generation,
					ProviderStatus:     providerStatus,
					LastOperation: &gardencorev1beta1.LastOperation{
						State:          lastOperationState,
						LastUpdateTime: metav1.NewTime(fakeClock.Now()),
					},
				},
			}
			if operationType != "" {
				switch operationType {
				case string(v1beta1constants.GardenerOperationReconcile):
					extensionBackupEntry.Status.LastOperation.Type = v1beta1constants.GardenerOperationReconcile
				case string(v1beta1constants.GardenerOperationRestore):
					extensionBackupEntry.Status.LastOperation.Type = v1beta1constants.GardenerOperationRestore
				}
			}
			ExpectWithOffset(1, testClient.Status().Patch(ctx, extensionBackupEntry, patch)).To(Succeed())

			if operationType != v1beta1constants.GardenerOperationReconcile {
				ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupEntry), extensionBackupEntry)).To(Succeed())
				patch := client.MergeFrom(extensionBackupEntry.DeepCopy())
				delete(extensionBackupEntry.Annotations, v1beta1constants.GardenerOperation)
				ExpectWithOffset(1, testClient.Patch(ctx, extensionBackupEntry, patch)).To(Succeed())
			}
		}

		reconcileCoreBackupBucket = func(makeReady bool) {
			ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket)).To(Succeed())
			patch := client.MergeFrom(backupBucket.DeepCopy())
			lastOperationState := gardencorev1beta1.LastOperationStateSucceeded
			if !makeReady {
				lastOperationState = gardencorev1beta1.LastOperationStateError
			}

			backupBucket.Status = gardencorev1beta1.BackupBucketStatus{
				ObservedGeneration: 1,
				LastOperation: &gardencorev1beta1.LastOperation{
					State: lastOperationState,
					Type:  gardencorev1beta1.LastOperationTypeReconcile,
				},
				ProviderStatus: providerStatus,
			}
			ExpectWithOffset(1, testClient.Status().Patch(ctx, backupBucket, patch)).To(Succeed())
		}

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
				GenerateName: "foo-",
				Labels:       map[string]string{testID: testRunID},
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
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), &gardencorev1beta1.BackupBucket{})
		}).Should(Succeed())

		DeferCleanup(func() {
			By("Delete BackupBucket")
			Expect(testClient.Delete(ctx, backupBucket)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Create Shoot")
		shootName := "shoot-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: testNamespace.Name,
				UID:       "foo",
			},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: ptr.To("test-sb"),
				CloudProfileName:  ptr.To("test-cloudprofile"),
				Region:            "foo-region",
				Provider: gardencorev1beta1.Provider{
					Type: "provider",
					Workers: []gardencorev1beta1.Worker{
						{
							Name:    "cpu-worker",
							Minimum: 2,
							Maximum: 2,
							Machine: gardencorev1beta1.Machine{
								Type: "large",
							},
						},
					},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.31.1",
				},
				Networking: &gardencorev1beta1.Networking{
					Type: ptr.To("foo-networking"),
				},
			},
		}
		shootTechnicalID = fmt.Sprintf("shoot--%s--%s", projectName, shootName)

		Expect(testClient.Create(ctx, shoot)).To(Succeed())
		log.Info("Created Shoot for test", "namespaceName", shoot.Name)

		By("Wait until manager has observed shoot")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), &gardencorev1beta1.Shoot{})
		}).Should(Succeed())

		DeferCleanup(func() {
			By("Delete shoot")
			Expect(testClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Create Shoot Namespace")
		shootNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: shootTechnicalID,
			},
		}

		Expect(testClient.Create(ctx, shootNamespace)).To(Succeed())
		log.Info("Created Shoot Namespace for test", "namespaceName", shootNamespace.Name)

		DeferCleanup(func() {
			By("Delete shoot namespace")
			Expect(testClient.Delete(ctx, shootNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Create Cluster resource")
		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootTechnicalID,
				Namespace: shootNamespace.Name,
			},
			Spec: extensionsv1alpha1.ClusterSpec{
				Shoot: runtime.RawExtension{
					Object: shoot,
				},
				Seed: runtime.RawExtension{
					Object: seed,
				},
				CloudProfile: runtime.RawExtension{
					Object: &gardencorev1beta1.CloudProfile{},
				},
			},
		}

		Expect(testClient.Create(ctx, cluster)).To(Succeed())
		log.Info("Created cluster for test", "cluster", client.ObjectKeyFromObject(cluster))

		By("Ensure manager cache observes cluster creation")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(cluster), &extensionsv1alpha1.Cluster{})
		}).Should(Succeed())

		DeferCleanup(func() {
			By("Delete cluster")
			Expect(testClient.Delete(ctx, cluster)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Create BackupEntry")
		backupEntry = &gardencorev1beta1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: shootTechnicalID + "--",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
				Annotations: utils.MergeStringMaps(annotations, map[string]string{
					v1beta1constants.ShootPurpose: shootPurpose,
				}),
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(shoot, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")),
				},
			},
			Spec: gardencorev1beta1.BackupEntrySpec{
				BucketName: backupBucket.Name,
				SeedName:   ptr.To(seed.Name),
			},
		}

		Expect(testClient.Create(ctx, backupEntry)).To(Succeed())
		log.Info("Created BackupEntry for test", "backupEntry", client.ObjectKeyFromObject(backupEntry))

		DeferCleanup(func() {
			By("Delete BackupEntry")
			Expect(testClient.Delete(ctx, backupEntry)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Create Shoot state in project namespace")
		shootState = &gardencorev1beta1.ShootState{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shoot.Name,
				Namespace: testNamespace.Name,
			},
			Spec: gardencorev1beta1.ShootStateSpec{
				Gardener: []gardencorev1beta1.GardenerResourceData{
					{
						Name: "data",
					},
				},
			},
		}

		Expect(testClient.Create(ctx, shootState)).To(Succeed())
		log.Info("Created Shoot state in project namespace for test", "shootState", client.ObjectKeyFromObject(shootState))

		DeferCleanup(func() {
			By("Delete Shoot state in project namespace")
			Expect(testClient.Delete(ctx, shootState)).To(Succeed())
		})

		extensionSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "entry-" + backupEntry.Name,
				Namespace: seedGardenNamespace.Name,
			},
		}

		extensionBackupEntry = &extensionsv1alpha1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: backupEntry.Name,
			},
		}

		By("Ensure finalizer got added")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)).To(Succeed())
			g.Expect(backupEntry.Finalizers).To(ConsistOf("gardener"))
		}).Should(Succeed())

		By("Mimic core BackupBucket condition")
		reconcileCoreBackupBucket(true)
	})

	Context("reconcile", func() {
		It("should set the BackupEntry status as Succeeded if the associated BackupBucket extension BackupEntry is ready", func() {
			By("Mimic extension backupEntry condition")
			reconcileExtensionBackupEntry(true)

			By("Ensure the BackupEntry status is set")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)).To(Succeed())
				g.Expect(backupEntry.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
				g.Expect(backupEntry.Status.LastError).To(BeNil())
				g.Expect(backupEntry.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				g.Expect(backupEntry.Status.LastOperation.Progress).To(Equal(int32(100)))
				g.Expect(backupEntry.Status.ObservedGeneration).To(Equal(backupEntry.Generation))
			}).Should(Succeed())
		})

		It("should set the BackupEntry status as Error if the associated BackupBucket condition is not ready", func() {
			By("Mimic core BackupBucket error condition")
			reconcileCoreBackupBucket(false)

			By("Ensure the BackupEntry status is set")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)).To(Succeed())
				g.Expect(backupEntry.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
				g.Expect(backupEntry.Status.LastError).NotTo(BeNil())
				g.Expect(backupEntry.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateError))
			}).Should(Succeed())
		})

		It("should set the BackupEntry status as Error if the extension BackupEntry is not ready", func() {
			By("Mimic extension backupEntry error condition")
			reconcileExtensionBackupEntry(false)

			By("Ensure the BackupEntry status is set")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)).To(Succeed())
				g.Expect(backupEntry.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
				g.Expect(backupEntry.Status.LastError).NotTo(BeNil())
				g.Expect(backupEntry.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateError))
			}).Should(Succeed())
		})
	})

	Context("migrate", func() {
		var targetSeed *gardencorev1beta1.Seed

		BeforeEach(func() {
			By("Create seed")
			targetSeed = &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "seed-",
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.SeedSpec{
					Provider: gardencorev1beta1.SeedProvider{
						Region: "region",
						Type:   "providerType",
					},
					Ingress: &gardencorev1beta1.Ingress{
						Domain: "seed.example.com",
						Controller: gardencorev1beta1.IngressController{
							Kind: "nginx",
						},
					},
					DNS: gardencorev1beta1.SeedDNS{
						Provider: &gardencorev1beta1.SeedDNSProvider{
							Type: "providerType",
							SecretRef: corev1.SecretReference{
								Name:      "some-secret",
								Namespace: "some-namespace",
							},
						},
					},
					Networks: gardencorev1beta1.SeedNetworks{
						Pods:     "10.0.0.0/16",
						Services: "10.1.0.0/16",
						Nodes:    ptr.To("10.2.0.0/16"),
					},
				},
			}
			Expect(testClient.Create(ctx, targetSeed)).To(Succeed())
			log.Info("Created target Seed for migration", "seed", targetSeed.Name)

			DeferCleanup(func() {
				By("Delete target seed")
				Expect(testClient.Delete(ctx, targetSeed)).To(Or(Succeed(), BeNotFoundError()))
			})
		})

		It("should set the BackupEntry status as Succeeded if the extension BackupEntry is migrated successfully", func() {
			By("Mimic extension backupEntry condition")
			reconcileExtensionBackupEntry(true)

			By("Ensure the BackupEntry status is set")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)).To(Succeed())
				g.Expect(backupEntry.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
				g.Expect(backupEntry.Status.LastError).To(BeNil())
				g.Expect(backupEntry.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				g.Expect(backupEntry.Status.LastOperation.Progress).To(Equal(int32(100)))
				g.Expect(backupEntry.Status.ObservedGeneration).To(Equal(backupEntry.Generation))
			}).Should(Succeed())

			By("Patch the seed name to trigger migration")
			patch := client.MergeFrom(backupEntry.DeepCopy())
			backupEntry.Spec.SeedName = &targetSeed.Name
			Expect(testClient.Patch(ctx, backupEntry, patch)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupEntry), extensionBackupEntry)).To(Succeed())
				g.Expect(extensionBackupEntry.Annotations).To(HaveKeyWithValue("gardener.cloud/operation", "migrate"))
			}).Should(Succeed())

			By("Patch the extension backupEntry to mimic successful migration")
			// These should be done by the extension controller, we are faking it here for the tests.
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupEntry), extensionBackupEntry)).To(Succeed())
			patch = client.MergeFrom(extensionBackupEntry.DeepCopy())
			extensionBackupEntry.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				Type:           gardencorev1beta1.LastOperationTypeMigrate,
				LastUpdateTime: metav1.NewTime(fakeClock.Now()),
			}
			Expect(testClient.Status().Patch(ctx, extensionBackupEntry, patch)).To(Succeed())

			patch = client.MergeFrom(extensionBackupEntry.DeepCopy())
			delete(extensionBackupEntry.Annotations, v1beta1constants.GardenerOperation)
			Expect(client.IgnoreNotFound(testClient.Patch(ctx, extensionBackupEntry, patch))).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupEntry), extensionBackupEntry)).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionSecret), extensionSecret)).To(BeNotFoundError())
			}).Should(Succeed())
		})
	})

	Context("restore", func() {
		BeforeEach(func() {
			annotations = map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationRestore}
		})

		It("should restore the BackupEntry", func() {
			By("Mimic extension backupEntry condition")
			reconcileExtensionBackupEntry(true)

			By("Ensure the BackupEntry status is set")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)).To(Succeed())
				g.Expect(backupEntry.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
				g.Expect(backupEntry.Status.LastError).To(BeNil())
				g.Expect(backupEntry.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeRestore))
				g.Expect(backupEntry.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				g.Expect(backupEntry.Status.LastOperation.Progress).To(Equal(int32(100)))
				g.Expect(backupEntry.Status.ObservedGeneration).To(Equal(backupEntry.Generation))
			}).Should(Succeed())
		})
	})

	Context("delete", func() {
		It("should delete the BackupEntry and cleanup the resources", func() {
			By("Mimic extension backupEntry condition")
			reconcileExtensionBackupEntry(true)

			By("Ensure the BackupEntry status is set")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)).To(Succeed())
				g.Expect(backupEntry.Status.LastError).To(BeNil())
				g.Expect(backupEntry.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				g.Expect(backupEntry.Status.LastOperation.Progress).To(Equal(int32(100)))
				g.Expect(backupEntry.Status.ObservedGeneration).To(Equal(backupEntry.Generation))
			}).Should(Succeed())

			By("Delete the BackupEntry")
			Expect(testClient.Delete(ctx, backupEntry)).To(Succeed())

			By("Ensure the BackupEntry is not deleted till the grace period is passed")
			Consistently(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)
			}).Should(Succeed())

			By("Step the clock to pass the grace period")
			fakeClock.Step((time.Duration(deletionGracePeriodHours)*time.Hour + time.Minute))
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)).To(Succeed())
			patch := client.MergeFrom(backupEntry.DeepCopy())
			metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
			Expect(testClient.Patch(ctx, backupEntry, patch)).To(Succeed())

			By("Ensure the extension resources are cleaned up")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupEntry), extensionBackupEntry)).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionSecret), extensionSecret)).To(BeNotFoundError())
			}).Should(Succeed())

			By("Ensure finalizers are removed and BackupBucket is released")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)
			}).Should(BeNotFoundError())
		})
	})
})
