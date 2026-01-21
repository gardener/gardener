// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	extensionsbackupentrycontroller "github.com/gardener/gardener/extensions/pkg/controller/backupentry"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	extensionsintegrationtest "github.com/gardener/gardener/test/integration/extensions/controller"
)

const (
	pollInterval        = time.Second
	pollTimeout         = 4 * time.Minute
	pollSevereThreshold = pollTimeout
)

var _ = Describe("BackupEntry", func() {
	var (
		mgr           manager.Manager
		testNamespace *corev1.Namespace

		backupEntrySecret          *corev1.Secret
		backupEntry                *extensionsv1alpha1.BackupEntry
		backupEntryObjectKey       client.ObjectKey
		backupEntrySecretObjectKey client.ObjectKey

		creationTimeIn string
	)

	BeforeEach(OncePerOrdered, func() {
		testShootUID := string(uuid.NewUUID())

		By("Create test Namespace")
		testNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				// create dedicated namespace for each test run, so that we can run multiple tests concurrently for stress tests
				GenerateName: "shoot--foo--",
				Annotations: map[string]string{
					v1beta1constants.ShootUID: testShootUID,
				},
			},
		}

		Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
		log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)

		DeferCleanup(func() {
			By("Delete test Namespace")
			Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})

		cluster := &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace.Name,
			},
			Spec: extensionsv1alpha1.ClusterSpec{
				CloudProfile: runtime.RawExtension{Raw: []byte("{}")},
				Seed:         runtime.RawExtension{Raw: []byte("{}")},
				Shoot:        runtime.RawExtension{Raw: []byte("{}")},
			},
		}

		By("Create Cluster")
		Expect(testClient.Create(ctx, cluster)).To(Succeed())
		log.Info("Created Cluster for test", "cluster", client.ObjectKeyFromObject(cluster))

		DeferCleanup(func() {
			By("Delete Cluster")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, cluster))).To(Succeed())
		})

		backupEntryName := testNamespace.Name + "--" + testShootUID

		backupEntrySecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "entry-" + backupEntryName,
				Namespace: testNamespace.Name,
			},
		}

		backupEntry = &extensionsv1alpha1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name:   backupEntryName,
				Labels: map[string]string{testID: testNamespace.Name},
			},
			Spec: extensionsv1alpha1.BackupEntrySpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: extensionsintegrationtest.Type,
				},
				SecretRef: corev1.SecretReference{
					Name:      backupEntrySecret.Name,
					Namespace: backupEntrySecret.Namespace,
				},
				Region: "foo",
			},
		}

		By("Setup manager")
		var err error
		mgr, err = manager.New(restConfig, manager.Options{
			Scheme:  kubernetes.SeedScheme,
			Metrics: metricsserver.Options{BindAddress: "0"},
			Cache: cache.Options{
				DefaultNamespaces: map[string]cache.Config{testNamespace.Name: {}},
				ByObject: map[client.Object]cache.ByObject{
					&extensionsv1alpha1.BackupEntry{}: {
						Label: labels.SelectorFromSet(labels.Set{testID: testNamespace.Name}),
					},
				},
			},
			Controller: controllerconfig.Controller{
				SkipNameValidation: ptr.To(true),
			},
		})
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(OncePerOrdered, func() {
		By("Start manager")
		mgrContext, mgrCancel := context.WithCancel(ctx)

		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(mgrContext)).To(Succeed())
		}()

		DeferCleanup(func() {
			By("Stop manager")
			mgrCancel()
		})

		By("Create secret for BackupEntry")
		Expect(testClient.Create(ctx, backupEntrySecret)).To(Succeed())
		backupEntrySecretObjectKey = client.ObjectKeyFromObject(backupEntrySecret)
		log.Info("Created Secret for BackupEntry for test", "secret", backupEntrySecretObjectKey)

		DeferCleanup(func() {
			By("Delete secret for BackupEntry")
			Expect(testClient.Get(ctx, backupEntrySecretObjectKey, backupEntrySecret)).To(Succeed())
			Expect(controllerutils.RemoveFinalizers(ctx, testClient, backupEntrySecret, extensionsbackupentrycontroller.FinalizerName)).To(Succeed())
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, backupEntrySecret))).To(Succeed())
			Eventually(func() error {
				return testClient.Get(ctx, backupEntrySecretObjectKey, backupEntrySecret)
			}).Should(BeNotFoundError())
		})

		By("Wait until manager has observed Namespace and Secret")
		Eventually(func(g Gomega) {
			// Wait until the manager's client has observed the creation of the Namespace and Secret before creating
			// the BackupEntry. Otherwise, the tests can be flaky because the manager could detect the BackupEntry
			// creation either after the creation of the Namespace and Secret, or before that. The latter causes an extra
			// reconciliation when the operation annotation is ignored.
			g.Expect(mgr.GetClient().Get(ctx, client.ObjectKeyFromObject(testNamespace), testNamespace)).To(Succeed())
			g.Expect(mgr.GetClient().Get(ctx, backupEntrySecretObjectKey, backupEntrySecret)).To(Succeed())
		}).Should(Succeed())

		By("Create BackupEntry")
		creationTimeIn = time.Now().String()
		metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, extensionsintegrationtest.AnnotationKeyTimeIn, creationTimeIn)
		Expect(testClient.Create(ctx, backupEntry)).To(Succeed())
		backupEntryObjectKey = client.ObjectKeyFromObject(backupEntry)
		log.Info("Created BackupEntry for test", "backupEntry", backupEntryObjectKey)

		DeferCleanup(func() {
			By("Delete BackupEntry")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, backupEntry))).To(Succeed())
			Eventually(func() error {
				return testClient.Get(ctx, backupEntryObjectKey, backupEntry)
			}).Should(BeNotFoundError())
		})
	})

	Describe("Create", func() {
		BeforeEach(OncePerOrdered, func() {
			Expect(addTestControllerToManagerWithOptions(mgr)).To(Succeed())
		})

		Context("BackupEntry is created successfully", Ordered, func() {
			It("should become ready", func() {
				Expect(waitForBackupEntryToBeReady(ctx, testClient, log, backupEntry)).To(Succeed())
			})

			It("should add finalizer to Secret for BackupEntry", func() {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntrySecret), backupEntrySecret)).To(Succeed())
				Expect(backupEntrySecret.Finalizers).To(ConsistOf(extensionsbackupentrycontroller.FinalizerName))
			})

			It("should have last operation with type Create and status Succeeded", func() {
				Expect(testClient.Get(ctx, backupEntryObjectKey, backupEntry)).To(Succeed())
				verifyBackupEntry(backupEntry, 1, 1, creationTimeIn, Equal(gardencorev1beta1.LastOperationTypeCreate), Equal(gardencorev1beta1.LastOperationStateSucceeded))
			})
		})

		Context("missing Secret for BackupEntry", Ordered, func() {
			BeforeAll(func() {
				backupEntry.Spec.SecretRef.Name = "missing-secret"
			})

			It("should set the last operation field on BackupEntry", func() {
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, backupEntryObjectKey, backupEntry)).To(Succeed())
					g.Expect(backupEntry.Status.LastOperation).ToNot(BeNil())
				}).Should(Succeed())
			})

			It("should verify that BackupEntry remains in processing state", func() {
				Consistently(func(g Gomega) {
					g.Expect(testClient.Get(ctx, backupEntryObjectKey, backupEntry)).To(Succeed())
					g.Expect(backupEntry.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateProcessing))
					g.Expect(backupEntry.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeCreate))
				}).Should(Succeed())
			})

			It("should successfully patch BackupEntry with correct secret reference", func() {
				Expect(patchBackupEntryObject(ctx, testClient, backupEntry, func() {
					backupEntry.Spec.SecretRef.Name = backupEntrySecret.Name
				})).To(Succeed())
			})

			It("should verify that BackupEntry becomes ready", func() {
				Expect(waitForBackupEntryToBeReady(ctx, testClient, log, backupEntry)).To(Succeed())
			})

			It("should add finalizer to Secret for BackupEntry", func() {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntrySecret), backupEntrySecret)).To(Succeed())
				Expect(backupEntrySecret.Finalizers).To(ConsistOf(extensionsbackupentrycontroller.FinalizerName))
			})

			It("should verify that BackupEntry has operation with type Create and status Succeeded", func() {
				Expect(testClient.Get(ctx, backupEntryObjectKey, backupEntry)).To(Succeed())
				verifyBackupEntry(backupEntry, 2, 2, creationTimeIn, Equal(gardencorev1beta1.LastOperationTypeCreate), Equal(gardencorev1beta1.LastOperationStateSucceeded))
			})
		})
	})

	Describe("Reconcile", func() {
		JustBeforeEach(OncePerOrdered, func() {
			By("Wait for BackupEntry to be created successfully")
			Expect(waitForBackupEntryToBeReady(ctx, testClient, log, backupEntry)).To(Succeed())
		})

		When("operation annotation is ignored", func() {
			var (
				// When the operation annotation is ignored then there is the Secret and Namespace mappers which may lead to multiple
				// reconciliations, hence we are okay with both Create/Reconcile last operation types.
				// Due to the same reason, the time when the BackupEntry is read it can be under reconciliation.
				// Still, the `Succeeded` state is ensured by each call of waitForBackupEntryToBeReady.
				lastOperationTypeMatcher  = Or(Equal(gardencorev1beta1.LastOperationTypeCreate), Equal(gardencorev1beta1.LastOperationTypeReconcile))
				lastOperationStateMatcher = Or(Equal(gardencorev1beta1.LastOperationStateSucceeded), Equal(gardencorev1beta1.LastOperationStateProcessing))
			)

			BeforeEach(OncePerOrdered, func() {
				// Add finalizers to BackupEntry Secret before it is created. Otherwise they will be added during the reconciliation
				// of the BackupEntry, which will cause it to be requeued for reconciliation due to the modification of the Secret.
				backupEntrySecret.Finalizers = append(backupEntrySecret.Finalizers, extensionsbackupentrycontroller.FinalizerName)
				Expect(addTestControllerToManagerWithOptions(mgr, ignoreOperationAnnotationOption(true))).To(Succeed())
			})

			When("BackupEntry is updated", Ordered, func() {
				var modificationTimeIn string

				It("should update BackupEntry", func() {
					modificationTimeIn = time.Now().String()

					Expect(patchBackupEntryObject(ctx, testClient, backupEntry, func() {
						metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, extensionsintegrationtest.AnnotationKeyTimeIn, modificationTimeIn)
						backupEntry.Spec.Region += "1"
					})).To(Succeed())
				})

				It("should verify that BackupEntry becomes ready", func() {
					Expect(waitForBackupEntryToBeReady(ctx, testClient, log, backupEntry)).To(Succeed())
				})

				It("should reconcile BackupEntry successfully", func() {
					Expect(testClient.Get(ctx, backupEntryObjectKey, backupEntry)).To(Succeed())
					verifyBackupEntry(backupEntry, 2, 2, modificationTimeIn, Equal(gardencorev1beta1.LastOperationTypeReconcile), lastOperationStateMatcher)
				})
			})

			When("Secret is updated", Ordered, func() {
				var modificationTimeIn string

				It("should overwrite time-in annotation on BackupEntry with current timestamp", func() {
					modificationTimeIn = time.Now().String()
					Expect(patchBackupEntryObject(ctx, testClient, backupEntry, func() {
						metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, extensionsintegrationtest.AnnotationKeyTimeIn, modificationTimeIn)
					})).To(Succeed())
				})

				It("should verify that BackupEntry is ready", func() {
					Expect(waitForBackupEntryToBeReady(ctx, testClient, log, backupEntry)).To(Succeed())
				})

				It("should verify that BackupEntry still has old time-out annotation because reconciliation has not yet occurred", func() {
					Expect(testClient.Get(ctx, backupEntryObjectKey, backupEntry)).To(Succeed())
					verifyBackupEntry(backupEntry, 1, 1, creationTimeIn, lastOperationTypeMatcher, lastOperationStateMatcher)
				})

				It("should generate event for Secret", func() {
					Expect(testClient.Get(ctx, backupEntrySecretObjectKey, backupEntrySecret)).To(Succeed())
					metav1.SetMetaDataAnnotation(&backupEntrySecret.ObjectMeta, "foo", "bar")
					Expect(testClient.Update(ctx, backupEntrySecret)).To(Succeed())
				})

				It("should verify that BackupEntry becomes ready after modification of Secret", func() {
					// wait for lastOperation's update time to be updated to give extension controller some time to observe
					// event and start reconciliation
					Expect(waitForBackupEntryToBeReady(ctx, testClient, log, backupEntry, backupEntry.Status.LastOperation.LastUpdateTime)).To(Succeed())
				})

				It("should verify that BackupEntry is reconciled after modification of Secret and has new time-in annotation", func() {
					Expect(testClient.Get(ctx, backupEntryObjectKey, backupEntry)).To(Succeed())
					verifyBackupEntry(backupEntry, 1, 1, modificationTimeIn, Equal(gardencorev1beta1.LastOperationTypeReconcile), lastOperationStateMatcher)
				})
			})

			When("Namespace is updated", Ordered, func() {
				var modificationTimeIn string

				It("should overwrite time-in annotation on BackupEntry with current timestamp", func() {
					modificationTimeIn = time.Now().String()
					Expect(patchBackupEntryObject(ctx, testClient, backupEntry, func() {
						metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, extensionsintegrationtest.AnnotationKeyTimeIn, modificationTimeIn)
					})).To(Succeed())
				})

				It("should verify that BackupEntry is ready", func() {
					Expect(waitForBackupEntryToBeReady(ctx, testClient, log, backupEntry)).To(Succeed())
				})

				It("should verify that BackupEntry still has old time-out annotation because reconciliation has not yet occurred", func() {
					Expect(testClient.Get(ctx, backupEntryObjectKey, backupEntry)).To(Succeed())
					verifyBackupEntry(backupEntry, 1, 1, creationTimeIn, lastOperationTypeMatcher, lastOperationStateMatcher)
				})

				It("should generate event for Namespace", func() {
					Expect(testClient.Get(ctx, client.ObjectKeyFromObject(testNamespace), testNamespace)).To(Succeed())
					metav1.SetMetaDataAnnotation(&testNamespace.ObjectMeta, "foo", "bar")
					Expect(testClient.Update(ctx, testNamespace)).To(Succeed())
				})

				It("should verify that BackupEntry becomes ready after modification of Namespace", func() {
					// wait for lastOperation's update time to be updated to give extension controller some time to observe
					// event and start reconciliation
					Expect(waitForBackupEntryToBeReady(ctx, testClient, log, backupEntry, backupEntry.Status.LastOperation.LastUpdateTime)).To(Succeed())
				})

				It("should verify that BackupEntry is reconciled after modification of Namespace and has new time-in annotation", func() {
					Expect(testClient.Get(ctx, backupEntryObjectKey, backupEntry)).To(Succeed())
					verifyBackupEntry(backupEntry, 1, 1, modificationTimeIn, Equal(gardencorev1beta1.LastOperationTypeReconcile), lastOperationStateMatcher)
				})
			})

			When("error occurs during reconciliation", Ordered, func() {
				It("should provoke reconciliation error for BackupEntry", func() {
					Expect(patchBackupEntryObject(ctx, testClient, backupEntry, func() {
						metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, extensionsintegrationtest.AnnotationKeyDesiredOperationState, extensionsintegrationtest.AnnotationValueDesiredOperationStateError)
						backupEntry.Spec.Region += "1"
					})).To(Succeed())
				})

				It("should verify BackupEntry status transitioned to error", func() {
					Eventually(func(g Gomega) {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)).To(Succeed())
						g.Expect(backupEntry.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeReconcile), fmt.Sprintf("BackupEntry %s operation is not reconcile", client.ObjectKeyFromObject(backupEntry).String()))
						g.Expect(backupEntry.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateError))
					}).Should(Succeed())
				})

				It("should fix reconciliation error for BackupEntry", func() {
					Expect(patchBackupEntryObject(ctx, testClient, backupEntry, func() {
						metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, extensionsintegrationtest.AnnotationKeyDesiredOperationState, "")
					})).To(Succeed())
				})

				It("should wait for BackupEntry to be ready", func() {
					Expect(waitForBackupEntryToBeReady(ctx, testClient, log, backupEntry)).To(Succeed())
				})

				It("should successfully reconcile BackupEntry", func() {
					Expect(testClient.Get(ctx, backupEntryObjectKey, backupEntry)).To(Succeed())
					verifyBackupEntry(backupEntry, 2, 2, creationTimeIn, Equal(gardencorev1beta1.LastOperationTypeReconcile), Equal(gardencorev1beta1.LastOperationStateSucceeded))
				})
			})
		})

		When("operation annotation is not ignored", func() {
			BeforeEach(OncePerOrdered, func() {
				Expect(addTestControllerToManagerWithOptions(mgr, ignoreOperationAnnotationOption(false))).To(Succeed())
			})

			When("BackupEntry is updated", Ordered, func() {
				var modificationTimeIn string

				It("should update BackupEntry", func() {
					modificationTimeIn = time.Now().String()
					Expect(patchBackupEntryObject(ctx, testClient, backupEntry, func() {
						metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, extensionsintegrationtest.AnnotationKeyTimeIn, modificationTimeIn)
						backupEntry.Spec.Region += "1"
					})).To(Succeed())
				})

				It("should not reconcile BackupEntry", func() {
					Expect(testClient.Get(ctx, backupEntryObjectKey, backupEntry)).To(Succeed())
					verifyBackupEntry(backupEntry, 2, 1, creationTimeIn, Equal(gardencorev1beta1.LastOperationTypeCreate), Equal(gardencorev1beta1.LastOperationStateSucceeded))
				})
			})

			When("BackupEntry is annotated with operation annotation", Ordered, func() {
				var modificationTimeIn string

				It("should annotate BackupEntry", func() {
					modificationTimeIn = time.Now().String()
					Expect(patchBackupEntryObject(ctx, testClient, backupEntry, func() {
						metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, extensionsintegrationtest.AnnotationKeyTimeIn, modificationTimeIn)
						metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
						backupEntry.Spec.Region += "1"
					})).To(Succeed())
				})

				It("should wait for BackupEntry to be ready", func() {
					Expect(waitForBackupEntryToBeReady(ctx, testClient, log, backupEntry)).To(Succeed())
				})

				It("should successfully reconcile BackupEntry", func() {
					Expect(testClient.Get(ctx, backupEntryObjectKey, backupEntry)).To(Succeed())
					verifyBackupEntry(backupEntry, 2, 2, modificationTimeIn, Equal(gardencorev1beta1.LastOperationTypeReconcile), Equal(gardencorev1beta1.LastOperationStateSucceeded))
					Expect(backupEntry.Annotations).ToNot(HaveKey(v1beta1constants.GardenerOperation))
				})
			})
		})

	})

	Describe("Delete", func() {
		BeforeEach(OncePerOrdered, func() {
			Expect(addTestControllerToManagerWithOptions(mgr)).To(Succeed())
		})

		JustBeforeEach(OncePerOrdered, func() {
			By("Wait for BackupEntry to be created successfully")
			Expect(waitForBackupEntryToBeReady(ctx, testClient, log, backupEntry)).To(Succeed())
		})

		When("deletion does not run into error", Ordered, func() {
			It("should delete BackupEntry", func() {
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, backupEntry))).To(Succeed())
			})

			It("should wait until BackupEntry and Secret are deleted", func() {
				Eventually(func() error {
					return testClient.Get(ctx, backupEntryObjectKey, backupEntry)
				}).Should(BeNotFoundError())

				Expect(testClient.Get(ctx, client.ObjectKey{Name: testNamespace.Name}, testNamespace)).To(Succeed())
				Expect(testNamespace.Annotations).To(HaveKeyWithValue(extensionsintegrationtest.AnnotationKeyDesiredOperation, extensionsintegrationtest.AnnotationValueOperationDelete))

				Expect(testClient.Get(ctx, backupEntrySecretObjectKey, backupEntrySecret)).To(Succeed())
				Expect(backupEntrySecret.Finalizers).NotTo(ConsistOf(extensionsbackupentrycontroller.FinalizerName))
			})
		})

		When("deletion runs into error", Ordered, func() {
			It("should provoke error in deletion", func() {
				Expect(patchBackupEntryObject(ctx, testClient, backupEntry, func() {
					metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, extensionsintegrationtest.AnnotationKeyDesiredOperationState, extensionsintegrationtest.AnnotationValueDesiredOperationStateError)
				})).To(Succeed())
			})

			It("should delete BackupEntry", func() {
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, backupEntry))).To(Succeed())
			})

			It("should transition to error", func() {
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)).To(Succeed())
					g.Expect(backupEntry.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeDelete))
					g.Expect(backupEntry.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateError))
				}).Should(Succeed())
			})

			It("should fix deletion error", func() {
				Expect(patchBackupEntryObject(ctx, testClient, backupEntry, func() {
					metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, extensionsintegrationtest.AnnotationKeyDesiredOperationState, "")
				})).To(Succeed())
			})

			It("should wait until BackupEntry and Secret are deleted", func() {
				Eventually(func() error {
					return testClient.Get(ctx, backupEntryObjectKey, backupEntry)
				}).Should(BeNotFoundError())

				Expect(testClient.Get(ctx, client.ObjectKey{Name: testNamespace.Name}, testNamespace)).To(Succeed())
				Expect(testNamespace.Annotations).To(HaveKeyWithValue(extensionsintegrationtest.AnnotationKeyDesiredOperation, extensionsintegrationtest.AnnotationValueOperationDelete))

				Expect(testClient.Get(ctx, backupEntrySecretObjectKey, backupEntrySecret)).To(Succeed())
				Expect(backupEntrySecret.Finalizers).NotTo(ConsistOf(extensionsbackupentrycontroller.FinalizerName))
			})
		})

		When("deletion takes a long time and Secret for BackupEntry is modified", Ordered, func() {
			It("should add annotation to BackupEntry so that Secret is modified during deletion ", func() {
				Expect(patchBackupEntryObject(ctx, testClient, backupEntry, func() {
					metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, extensionsintegrationtest.AnnotationKeyDesiredOperation, "ModifySecretDuringDeletion")
				})).To(Succeed())
			})

			It("should delete BackupEntry", func() {
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, backupEntry))).To(Succeed())
			})

			It("should successfully finish deletion of BackupEntry", func() {
				Eventually(func() error {
					return testClient.Get(ctx, backupEntryObjectKey, backupEntry)
				}).Should(BeNotFoundError())

				Expect(testClient.Get(ctx, client.ObjectKey{Name: testNamespace.Name}, testNamespace)).To(Succeed())
				Expect(testNamespace.Annotations[extensionsintegrationtest.AnnotationKeyDesiredOperation]).To(Equal(extensionsintegrationtest.AnnotationValueOperationDelete))

				Expect(testClient.Get(ctx, backupEntrySecretObjectKey, backupEntrySecret)).To(Succeed())
				Expect(backupEntrySecret.Finalizers).NotTo(ConsistOf(extensionsbackupentrycontroller.FinalizerName))
			})
		})
	})

	Describe("Restore", func() {
		// Last operation type could be either Restore or Reconcile as reconciliation can be immediately triggered after successful restoration
		// because the `gardener.cloud/operation=restore` annotation is not removed at the start of the operation.
		var lastOperationTypeMatcher = Or(Equal(gardencorev1beta1.LastOperationTypeRestore), Equal(gardencorev1beta1.LastOperationTypeReconcile))

		BeforeEach(OncePerOrdered, func() {
			Expect(addTestControllerToManagerWithOptions(mgr)).To(Succeed())
		})

		JustBeforeEach(OncePerOrdered, func() {
			By("Wait for BackupEntry to be created successfully")
			Expect(waitForBackupEntryToBeReady(ctx, testClient, log, backupEntry)).To(Succeed())
		})

		When("restoration does not run into error", Ordered, func() {
			var modificationTimeIn string

			It("should annotate BackupEntry", func() {
				modificationTimeIn = time.Now().String()
				Expect(patchBackupEntryObject(ctx, testClient, backupEntry, func() {
					metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, extensionsintegrationtest.AnnotationKeyTimeIn, modificationTimeIn)
					metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRestore)
				})).To(Succeed())
			})

			It("should wait for BackupEntry to be ready", func() {
				Expect(waitForBackupEntryToBeReady(ctx, testClient, log, backupEntry)).To(Succeed())
			})

			It("should successfully restore BackupEntry", func() {
				Expect(testClient.Get(ctx, backupEntryObjectKey, backupEntry)).To(Succeed())
				verifyBackupEntry(backupEntry, 1, 1, modificationTimeIn, lastOperationTypeMatcher, Equal(gardencorev1beta1.LastOperationStateSucceeded))

				Expect(backupEntry.Annotations).To(And(
					Not(HaveKey(v1beta1constants.GardenerOperation)),
					HaveKeyWithValue(extensionsintegrationtest.AnnotationKeyDesiredOperation, v1beta1constants.GardenerOperationRestore),
				))
			})
		})

		When("restoration runs into error", Ordered, func() {
			It("should provoke restoration error", func() {
				Expect(patchBackupEntryObject(ctx, testClient, backupEntry, func() {
					metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, extensionsintegrationtest.AnnotationKeyDesiredOperationState, extensionsintegrationtest.AnnotationValueDesiredOperationStateError)
					metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRestore)
				})).To(Succeed())
			})

			It("should verify BackupEntry status transitioned to error", func() {
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)).To(Succeed())
					g.Expect(backupEntry.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeRestore))
					g.Expect(backupEntry.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateError))
				}).Should(Succeed())
			})

			It("should fix restoration error", func() {
				Expect(patchBackupEntryObject(ctx, testClient, backupEntry, func() {
					metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, extensionsintegrationtest.AnnotationKeyDesiredOperationState, "")
				})).To(Succeed())
			})

			It("should wait for BackupEntry to be ready", func() {
				Expect(waitForBackupEntryToBeReady(ctx, testClient, log, backupEntry)).To(Succeed())
			})

			It("should successfully restore BackupEntry", func() {
				Expect(testClient.Get(ctx, backupEntryObjectKey, backupEntry)).To(Succeed())
				verifyBackupEntry(backupEntry, 1, 1, creationTimeIn, lastOperationTypeMatcher, Equal(gardencorev1beta1.LastOperationStateSucceeded))

				Expect(backupEntry.Annotations).To(And(
					Not(HaveKey(v1beta1constants.GardenerOperation)),
					HaveKeyWithValue(extensionsintegrationtest.AnnotationKeyDesiredOperation, v1beta1constants.GardenerOperationRestore),
				))
			})
		})
	})

	Describe("Migrate", func() {
		BeforeEach(OncePerOrdered, func() {
			Expect(addTestControllerToManagerWithOptions(mgr)).To(Succeed())
		})

		JustBeforeEach(OncePerOrdered, func() {
			By("Wait for BackupEntry to be created successfully")
			Expect(waitForBackupEntryToBeReady(ctx, testClient, log, backupEntry)).To(Succeed())
		})

		When("migration does not run into error", Ordered, func() {
			var modificationTimeIn string

			It("should annotate BackupEntry", func() {
				modificationTimeIn = time.Now().String()
				Expect(patchBackupEntryObject(ctx, testClient, backupEntry, func() {
					metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, extensionsintegrationtest.AnnotationKeyTimeIn, modificationTimeIn)
					metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationMigrate)
				})).To(Succeed())
			})

			It("should wait for BackupEntry to be ready", func() {
				Expect(waitForBackupEntryToBeReady(ctx, testClient, log, backupEntry)).To(Succeed())
			})

			It("should successfully migrate BackupEntry", func() {
				Expect(testClient.Get(ctx, backupEntryObjectKey, backupEntry)).To(Succeed())
				Expect(backupEntry.Finalizers).To(BeEmpty())

				Expect(backupEntry.Annotations).To(And(
					Not(HaveKey(v1beta1constants.GardenerOperation)),
					HaveKeyWithValue(extensionsintegrationtest.AnnotationKeyDesiredOperation, v1beta1constants.GardenerOperationMigrate),
				))

				Expect(testClient.Get(ctx, backupEntrySecretObjectKey, backupEntrySecret)).To(Succeed())
				Expect(backupEntrySecret.Finalizers).To(BeEmpty())
			})
		})

		When("migration runs into error", Ordered, func() {
			It("should provoke migration error", func() {
				Expect(patchBackupEntryObject(ctx, testClient, backupEntry, func() {
					metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, extensionsintegrationtest.AnnotationKeyDesiredOperationState, extensionsintegrationtest.AnnotationValueDesiredOperationStateError)
					metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationMigrate)
				})).To(Succeed())
			})

			It("should verify BackupEntry status transitioned to error", func() {
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)).To(Succeed())
					g.Expect(backupEntry.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeMigrate))
					g.Expect(backupEntry.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateError))
				}).Should(Succeed())
			})

			It("should fix migration error", func() {
				Expect(patchBackupEntryObject(ctx, testClient, backupEntry, func() {
					metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, extensionsintegrationtest.AnnotationKeyDesiredOperationState, "")
				})).To(Succeed())
			})

			It("should wait for BackupEntry to be ready", func() {
				Expect(waitForBackupEntryToBeReady(ctx, testClient, log, backupEntry)).To(Succeed())
			})

			It("should successfully migrate BackupEntry", func() {
				Expect(testClient.Get(ctx, backupEntryObjectKey, backupEntry)).To(Succeed())
				Expect(backupEntry.Finalizers).To(BeEmpty())

				Expect(backupEntry.Annotations).To(And(
					Not(HaveKey(v1beta1constants.GardenerOperation)),
					HaveKeyWithValue(extensionsintegrationtest.AnnotationKeyDesiredOperation, v1beta1constants.GardenerOperationMigrate),
				))

				Expect(testClient.Get(ctx, backupEntrySecretObjectKey, backupEntrySecret)).To(Succeed())
				Expect(backupEntrySecret.Finalizers).To(BeEmpty())
			})
		})
	})
})

func patchBackupEntryObject(ctx context.Context, c client.Client, backupEntry *extensionsv1alpha1.BackupEntry, transform func()) error {
	if err := c.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry); err != nil {
		return err
	}

	patch := client.MergeFrom(backupEntry.DeepCopy())
	transform()
	return c.Patch(ctx, backupEntry, patch)
}

func waitForBackupEntryToBeReady(ctx context.Context, c client.Client, log logr.Logger, backupEntry *extensionsv1alpha1.BackupEntry, minOperationUpdateTime ...metav1.Time) error {
	healthFuncs := []health.Func{health.CheckExtensionObject}
	if len(minOperationUpdateTime) > 0 {
		healthFuncs = append(healthFuncs, health.ExtensionOperationHasBeenUpdatedSince(minOperationUpdateTime[0]))
	}

	return extensions.WaitUntilObjectReadyWithHealthFunction(
		ctx,
		c,
		log,
		health.And(healthFuncs...),
		backupEntry,
		extensionsv1alpha1.BackupEntryResource,
		pollInterval,
		pollSevereThreshold,
		pollTimeout,
		nil,
	)
}

func verifyBackupEntry(backupEntry *extensionsv1alpha1.BackupEntry, generation, observedGeneration int64, expectedTimeOut string, expectedLastOperationType, expectedLastOperationState gomegatypes.GomegaMatcher) {
	GinkgoHelper()

	Expect(backupEntry.Generation).To(Equal(generation), "generation")
	Expect(backupEntry.Finalizers).To(ConsistOf(extensionsbackupentrycontroller.FinalizerName))
	Expect(backupEntry.Status.LastOperation.Type).To(expectedLastOperationType)
	Expect(backupEntry.Status.LastOperation.State).To(expectedLastOperationState)
	Expect(backupEntry.Status.ObservedGeneration).To(Equal(observedGeneration), "observedGeneration")
	Expect(backupEntry.Annotations[extensionsintegrationtest.AnnotationKeyTimeOut]).To(Equal(expectedTimeOut))
}
