// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package selfhosted_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Self-hosted Shoot Care controller tests", func() {
	var (
		shoot   *gardencorev1beta1.Shoot
		cluster *extensionsv1alpha1.Cluster

		shootUID = types.UID("some-uid")
	)

	BeforeEach(func() {
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: v1beta1constants.GardenNamespace,
				Labels:    map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ShootSpec{
				CloudProfileName: ptr.To("cloudprofile1"),
				Region:           "europe-central-1",
				Provider: gardencorev1beta1.Provider{
					Type: "foo-provider",
					Workers: []gardencorev1beta1.Worker{
						{
							Name:    "cp-worker",
							Minimum: 1,
							Maximum: 1,
							Machine: gardencorev1beta1.Machine{
								Type: "large",
								Image: &gardencorev1beta1.ShootMachineImage{
									Name:    "some-image",
									Version: ptr.To("1.0.0"),
								},
							},
							ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
						},
					},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.31.1",
				},
				Networking: &gardencorev1beta1.Networking{
					Type:     ptr.To("foo-networking"),
					Services: ptr.To("10.0.0.0/16"),
					Pods:     ptr.To("10.1.0.0/16"),
				},
			},
		}
		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{testID: testRunID},
			},
			Spec: extensionsv1alpha1.ClusterSpec{
				Shoot:        runtime.RawExtension{Object: shoot},
				Seed:         runtime.RawExtension{Object: &gardencorev1beta1.Seed{}},
				CloudProfile: runtime.RawExtension{Object: &gardencorev1beta1.CloudProfile{}},
			},
		}
	})

	JustBeforeEach(func() {
		By("Create Shoot")
		Expect(testClient.Create(ctx, shoot)).To(Succeed())
		log.Info("Created Shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

		By("Patch shoot status")
		patch := client.MergeFrom(shoot.DeepCopy())
		shoot.Status.Gardener.Version = "1.2.3"
		shoot.Status.TechnicalID = cpNamespace.Name
		shoot.Status.UID = shootUID
		Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

		By("Ensure manager has observed status patch")
		Eventually(func(g Gomega) string {
			g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			return shoot.Status.Gardener.Version
		}).ShouldNot(BeEmpty())

		DeferCleanup(func() {
			By("Delete Shoot")
			Expect(testClient.Delete(ctx, shoot)).To(Succeed())

			By("Ensure Shoot is gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
			}).Should(BeNotFoundError())
		})
	})

	Context("when operation cannot be initialized", func() {
		// No Cluster resource is created, so the operation builder cannot find the Cluster
		// and will set all conditions to Unknown.
		It("should set all conditions including BackupBucketsReady to Unknown", func() {
			By("Expect conditions to be Unknown")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Status.Conditions
			}).Should(And(
				ContainCondition(OfType(gardencorev1beta1.ShootAPIServerAvailable), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("operation could not be initialized")),
				ContainCondition(OfType(gardencorev1beta1.ShootControlPlaneHealthy), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("operation could not be initialized")),
				ContainCondition(OfType(gardencorev1beta1.ShootObservabilityComponentsHealthy), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("operation could not be initialized")),
				ContainCondition(OfType(gardencorev1beta1.ShootEveryNodeReady), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("operation could not be initialized")),
				ContainCondition(OfType(gardencorev1beta1.ShootSystemComponentsHealthy), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("operation could not be initialized")),
				ContainCondition(OfType(gardencorev1beta1.SeedBackupBucketsReady), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("operation could not be initialized")),
			))
		})
	})

	Context("when operation can be initialized", func() {
		JustBeforeEach(func() {
			By("Create Cluster")
			// For self-hosted shoots, ControlPlaneNamespaceForShoot always returns "kube-system",
			// so the Cluster resource must be named "kube-system" regardless of the TechnicalID.
			cluster.Name = metav1.NamespaceSystem
			Expect(testClient.Create(ctx, cluster)).To(Succeed())
			log.Info("Created Cluster for test", "cluster", cluster.Name)

			DeferCleanup(func() {
				By("Delete Cluster")
				Expect(testClient.Delete(ctx, cluster)).To(Succeed())

				By("Ensure Cluster is gone")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)
				}).Should(BeNotFoundError())
			})
		})

		Context("BackupBucketsReady condition", func() {
			It("should set BackupBucketsReady=True/NoBackupEntry when no BackupEntry exists", func() {
				By("Expect BackupBucketsReady to be True with NoBackupEntry")
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					return shoot.Status.Conditions
				}).Should(ContainCondition(
					OfType(gardencorev1beta1.SeedBackupBucketsReady),
					WithStatus(gardencorev1beta1.ConditionTrue),
					WithReason("NoBackupEntry"),
				))
			})

			It("should set BackupBucketsReady=True when BackupBucket is healthy", func() {
				// For self-hosted shoots, ControlPlaneNamespaceForShoot always returns "kube-system",
				// so BackupEntryName is derived from "kube-system".
				backupEntryName, err := gardenerutils.GenerateBackupEntryName(metav1.NamespaceSystem, shootUID, shoot.UID)
				Expect(err).NotTo(HaveOccurred())

				backupBucket := &gardencorev1beta1.BackupBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "bb-" + testRunID,
						Labels: map[string]string{testID: testRunID},
					},
					Spec: gardencorev1beta1.BackupBucketSpec{
						Provider: gardencorev1beta1.BackupBucketProvider{
							Type:   "foo",
							Region: "europe",
						},
						SeedName: ptr.To("some-seed"),
						CredentialsRef: &corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Namespace:  v1beta1constants.GardenNamespace,
							Name:       "some-credentials",
						},
					},
				}

				By("Create BackupBucket")
				Expect(testClient.Create(ctx, backupBucket)).To(Succeed())

				backupEntry := &gardencorev1beta1.BackupEntry{
					ObjectMeta: metav1.ObjectMeta{
						Name:      backupEntryName,
						Namespace: v1beta1constants.GardenNamespace,
						Labels:    map[string]string{testID: testRunID},
					},
					Spec: gardencorev1beta1.BackupEntrySpec{
						BucketName: backupBucket.Name,
						SeedName:   ptr.To("some-seed"),
					},
				}

				By("Create BackupEntry")
				Expect(testClient.Create(ctx, backupEntry)).To(Succeed())

				DeferCleanup(func() {
					By("Delete BackupEntry")
					Expect(testClient.Delete(ctx, backupEntry)).To(Succeed())
					Eventually(func() error {
						return mgrClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)
					}).Should(BeNotFoundError())

					By("Delete BackupBucket")
					Expect(testClient.Delete(ctx, backupBucket)).To(Succeed())
					Eventually(func() error {
						return mgrClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket)
					}).Should(BeNotFoundError())
				})

				By("Expect BackupBucketsReady to be True")
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					return shoot.Status.Conditions
				}).Should(ContainCondition(
					OfType(gardencorev1beta1.SeedBackupBucketsReady),
					WithStatus(gardencorev1beta1.ConditionTrue),
					WithReason("BackupBucketsAvailable"),
				))
			})

			It("should set BackupBucketsReady=Progressing when BackupBucket is erroneous", func() {
				// For self-hosted shoots, ControlPlaneNamespaceForShoot always returns "kube-system",
				// so BackupEntryName is derived from "kube-system".
				backupEntryName, err := gardenerutils.GenerateBackupEntryName(metav1.NamespaceSystem, shootUID, shoot.UID)
				Expect(err).NotTo(HaveOccurred())

				errMsg := "some error message"
				backupBucket := &gardencorev1beta1.BackupBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "bb-err-" + testRunID,
						Labels: map[string]string{testID: testRunID},
					},
					Spec: gardencorev1beta1.BackupBucketSpec{
						Provider: gardencorev1beta1.BackupBucketProvider{
							Type:   "foo",
							Region: "europe",
						},
						SeedName: ptr.To("some-seed"),
						CredentialsRef: &corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Namespace:  v1beta1constants.GardenNamespace,
							Name:       "some-credentials",
						},
					},
				}

				By("Create BackupBucket")
				Expect(testClient.Create(ctx, backupBucket)).To(Succeed())

				By("Patch BackupBucket to report error")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket)).To(Succeed())
					patch := client.MergeFrom(backupBucket.DeepCopy())
					backupBucket.Status.LastError = &gardencorev1beta1.LastError{
						Description: errMsg,
					}
					g.Expect(testClient.Status().Patch(ctx, backupBucket, patch)).To(Succeed())
				}).Should(Succeed())

				backupEntry := &gardencorev1beta1.BackupEntry{
					ObjectMeta: metav1.ObjectMeta{
						Name:      backupEntryName,
						Namespace: v1beta1constants.GardenNamespace,
						Labels:    map[string]string{testID: testRunID},
					},
					Spec: gardencorev1beta1.BackupEntrySpec{
						BucketName: backupBucket.Name,
						SeedName:   ptr.To("some-seed"),
					},
				}

				By("Create BackupEntry")
				Expect(testClient.Create(ctx, backupEntry)).To(Succeed())

				DeferCleanup(func() {
					By("Delete BackupEntry")
					Expect(testClient.Delete(ctx, backupEntry)).To(Succeed())
					Eventually(func() error {
						return mgrClient.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry)
					}).Should(BeNotFoundError())

					By("Delete BackupBucket")
					Expect(testClient.Delete(ctx, backupBucket)).To(Succeed())
					Eventually(func() error {
						return mgrClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket)
					}).Should(BeNotFoundError())
				})

				By("Expect BackupBucketsReady to report the error")
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					return shoot.Status.Conditions
				}).Should(ContainCondition(
					OfType(gardencorev1beta1.SeedBackupBucketsReady),
					WithStatus(gardencorev1beta1.ConditionProgressing),
					WithReason("BackupBucketsError"),
					WithMessageSubstrings(fmt.Sprintf("Error: %s", errMsg)),
				))
			})
		})
	})
})
