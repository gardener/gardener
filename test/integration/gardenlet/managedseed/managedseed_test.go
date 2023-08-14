// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package managedseed_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenletv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ManagedSeed controller test", func() {
	var (
		shoot                            *gardencorev1beta1.Shoot
		managedSeed                      *seedmanagementv1alpha1.ManagedSeed
		shootKubeconfigSecret            *corev1.Secret
		shootSecretBinding               *gardencorev1beta1.SecretBinding
		shootCloudProviderSecret         *corev1.Secret
		backupSecret                     *corev1.Secret
		backupSecretName, seedSecretName string

		reconcileShoot = func() {
			By("Patch the Shoot as Reconciled")
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.ObservedGeneration = shoot.Generation
			shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			ExpectWithOffset(1, testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())
		}

		checkIfSeedSecretsCreated = func() {
			By("Verify if seed secrets are created")
			EventuallyWithOffset(1, func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(backupSecret), backupSecret)).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: seedSecretName, Namespace: gardenNamespaceGarden.Name}, &corev1.Secret{})).To(Succeed())
			}).Should(Succeed())
		}

		checkIfGardenletWasDeployed = func() {
			By("Verify if gardenlet is deployed")
			EventuallyWithOffset(1, func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:apiserver-sni"}, &rbacv1.ClusterRole{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet"}, &rbacv1.ClusterRole{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:managed-istio"}, &rbacv1.ClusterRole{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:apiserver-sni"}, &rbacv1.ClusterRoleBinding{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet"}, &rbacv1.ClusterRoleBinding{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:managed-istio"}, &rbacv1.ClusterRoleBinding{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener-system-critical"}, &schedulingv1.PriorityClass{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet", Namespace: gardenNamespaceShoot}, &rbacv1.Role{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet", Namespace: gardenNamespaceShoot}, &rbacv1.RoleBinding{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet-kubeconfig-bootstrap", Namespace: gardenNamespaceShoot}, &corev1.Secret{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet", Namespace: gardenNamespaceShoot}, &corev1.Service{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet", Namespace: gardenNamespaceShoot}, &corev1.ServiceAccount{})).To(Succeed())

				gardenletDeployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardenlet", Namespace: gardenNamespaceShoot}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(gardenletDeployment), gardenletDeployment)).To(Succeed())
				g.Expect(gardenletDeployment.Spec.Template.Annotations).To(HaveKeyWithValue(
					"checksum/seed-backup-secret", backupSecret.Name+"-"+utils.ComputeSecretChecksum(backupSecret.Data)[:8],
				))
			}).Should(Succeed())
		}
	)

	BeforeEach(func() {
		Eventually(func(g Gomega) {
			g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(gardenNamespaceGarden), &corev1.Namespace{})).To(Succeed())
		}).Should(Succeed())

		backupSecretName = "backup-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]
		seedSecretName = "seed-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]

		backupSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      backupSecretName,
				Namespace: gardenNamespaceGarden.Name,
			},
		}

		gardenletConfig, err := encoding.EncodeGardenletConfiguration(&gardenletv1alpha1.GardenletConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
				Kind:       "GardenletConfiguration",
			},
			GardenClientConnection: &gardenletv1alpha1.GardenClientConnection{
				KubeconfigSecret: &corev1.SecretReference{
					Name:      "gardenlet-kubeconfig",
					Namespace: gardenNamespaceGarden.Name,
				},
			},
			SeedConfig: &gardenletv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"bar": "baz",
						},
					},
					Spec: gardencorev1beta1.SeedSpec{
						Backup: &gardencorev1beta1.SeedBackup{
							Provider: "test",
							Region:   pointer.String("bar"),
							SecretRef: corev1.SecretReference{
								Name:      backupSecret.Name,
								Namespace: backupSecret.Namespace,
							},
						},
						SecretRef: &corev1.SecretReference{
							Name:      seedSecretName,
							Namespace: gardenNamespaceGarden.Name,
						},
					},
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		managedSeed = &seedmanagementv1alpha1.ManagedSeed{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "managedseed-",
				Namespace:    gardenNamespaceGarden.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: seedmanagementv1alpha1.ManagedSeedSpec{
				Gardenlet: &seedmanagementv1alpha1.Gardenlet{
					Deployment: &seedmanagementv1alpha1.GardenletDeployment{
						ReplicaCount:         pointer.Int32(1),
						RevisionHistoryLimit: pointer.Int32(1),
						Image: &seedmanagementv1alpha1.Image{
							PullPolicy: pullPolicyPtr(corev1.PullIfNotPresent),
						},
						VPA: pointer.Bool(false),
					},
					Config:    *gardenletConfig,
					Bootstrap: bootstrapPtr(seedmanagementv1alpha1.BootstrapToken),
				},
			},
		}

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: gardenNamespaceGarden.Name,
				Labels:    map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName:         &seed.Name,
				CloudProfileName: "foo",
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version:                     "1.25.1",
					EnableStaticTokenKubeconfig: pointer.Bool(true),
				},
				Networking: &gardencorev1beta1.Networking{
					Type: pointer.String("foo"),
				},
				DNS: &gardencorev1beta1.DNS{
					Domain: pointer.String("replica-name.example.com"),
				},
				Provider: gardencorev1beta1.Provider{
					Type: "foo",
					Workers: []gardencorev1beta1.Worker{
						{
							Name: "some-worker",
							Machine: gardencorev1beta1.Machine{
								Type:         "some-machine-type",
								Architecture: pointer.String("amd64"),
							},
							Maximum: 2,
							Minimum: 1,
						},
					},
				},
				Region: "some-region",
			},
		}
	})

	JustBeforeEach(func() {
		By("Create cloud provider Secret for shoot")
		shootCloudProviderSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    gardenNamespaceGarden.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}
		Expect(testClient.Create(ctx, shootCloudProviderSecret)).To(Succeed())
		log.Info("Created cloud provider Secret for shoot", "secret", client.ObjectKeyFromObject(shootCloudProviderSecret))

		DeferCleanup(func() {
			By("Delete cloud provider Secret")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shootCloudProviderSecret))).To(Succeed())

			By("Wait for cloud provider Secret to be gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(shootCloudProviderSecret), shootCloudProviderSecret)
			}).Should(BeNotFoundError())
		})

		By("Create SecretBinding for shoot")
		shootSecretBinding = &gardencorev1beta1.SecretBinding{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    gardenNamespaceGarden.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Provider: &gardencorev1beta1.SecretBindingProvider{
				Type: "providerType",
			},
			SecretRef: corev1.SecretReference{
				Name:      shootCloudProviderSecret.Name,
				Namespace: shootCloudProviderSecret.Namespace,
			},
		}
		Expect(testClient.Create(ctx, shootSecretBinding)).To(Succeed())
		log.Info("Created SecretBinding for shoot", "secretbinding", client.ObjectKeyFromObject(shootSecretBinding))

		DeferCleanup(func() {
			By("Delete shoot SecretBinding")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shootSecretBinding))).To(Succeed())

			By("Wait for shoot SecretBinding to be gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(shootSecretBinding), shootSecretBinding)
			}).Should(BeNotFoundError())
		})

		By("Create kubeconfig Secret for shoot")
		shootKubeconfigSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shoot.Name + ".kubeconfig",
				Namespace: shoot.Namespace,
				Labels:    map[string]string{testID: testRunID},
			},
			Data: map[string][]byte{
				"kubeconfig": []byte("kubeconfig"),
			},
		}
		Expect(testClient.Create(ctx, shootKubeconfigSecret)).To(Succeed())
		log.Info("Created kubeconfig Secret for shoot", "shootKubeconfigSecret", client.ObjectKeyFromObject(shootKubeconfigSecret))

		DeferCleanup(func() {
			By("Delete kubeconfig Secret")
			Expect(testClient.Delete(ctx, shootKubeconfigSecret)).To(Or(Succeed(), BeNotFoundError()))

			By("Wait for shoot kubeconfig Secret to be gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(shootKubeconfigSecret), shootKubeconfigSecret)
			}).Should(BeNotFoundError())
		})

		By("Create Shoot")
		shoot.Spec.SecretBindingName = pointer.String(shootSecretBinding.Name)
		Expect(testClient.Create(ctx, shoot)).To(Succeed())
		log.Info("Created Shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

		By("Ensure Shoot is created")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
		}).Should(Succeed())

		DeferCleanup(func() {
			By("Delete Shoot")
			Expect(testClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))

			By("Wait for Shoot to be gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
			}).Should(BeNotFoundError())
		})

		By("Create ManagedSeed")
		managedSeed.Spec.Shoot = &seedmanagementv1alpha1.Shoot{Name: shoot.Name}
		Expect(testClient.Create(ctx, managedSeed)).To(Succeed())
		log.Info("Created ManagedSeed for test", "managedseed", client.ObjectKeyFromObject(managedSeed))

		DeferCleanup(func() {
			By("Delete ManagedSeed")
			Expect(testClient.Delete(ctx, managedSeed)).To(Or(Succeed(), BeNotFoundError()))

			By("Wait for ManagedSeed to be gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)
			}).Should(BeNotFoundError())
		})

		By("Ensure finalizer is added")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
			g.Expect(managedSeed.Finalizers).To(ConsistOf("gardener"))
		}).Should(Succeed())
	})

	Context("shoot not reconciled", func() {
		It("should set the ShootReconciled status of ManagedSeed to false because the shoot is not yet reconciled", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
				condition := v1beta1helper.GetCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedShootReconciled)
				g.Expect(condition).NotTo(BeNil())
				g.Expect(condition.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				g.Expect(condition.Reason).To(Equal(gardencorev1beta1.EventReconciling))
			}).Should(Succeed())
		})
	})

	Context("shoot reconciled", func() {
		JustBeforeEach(func() {
			reconcileShoot()
		})

		It("should set the ShootReconciled status to true,create seed secrets specified in spec.backup.secretRef and spec.secretRef field of seed template and deploy gardenlet ", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
				condition := v1beta1helper.GetCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedShootReconciled)
				g.Expect(condition).NotTo(BeNil())
				g.Expect(condition.Status).To(Equal(gardencorev1beta1.ConditionTrue))
				g.Expect(condition.Reason).To(Equal(gardencorev1beta1.EventReconciled))
			}).Should(Succeed())

			checkIfSeedSecretsCreated()
			checkIfGardenletWasDeployed()
		})

		It("should delete the seed secret when .spec.secretRef is unset", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
				condition := v1beta1helper.GetCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedShootReconciled)
				g.Expect(condition).NotTo(BeNil())
				g.Expect(condition.Status).To(Equal(gardencorev1beta1.ConditionTrue))
				g.Expect(condition.Reason).To(Equal(gardencorev1beta1.EventReconciled))
			}).Should(Succeed())

			checkIfSeedSecretsCreated()
			checkIfGardenletWasDeployed()

			patch := client.MergeFrom(managedSeed.DeepCopy())
			gardenletConfig, err := encoding.DecodeGardenletConfiguration(&managedSeed.Spec.Gardenlet.Config, false)
			Expect(err).NotTo(HaveOccurred())
			gardenletConfig.SeedConfig.Spec.SecretRef = nil
			gardenletConfigRaw, err := encoding.EncodeGardenletConfiguration(gardenletConfig)
			Expect(err).NotTo(HaveOccurred())
			managedSeed.Spec.Gardenlet.Config = *gardenletConfigRaw
			// This should be ideally done by the ManagedSeed admission plugin, but it's disabled in the test
			metav1.SetMetaDataAnnotation(&managedSeed.ObjectMeta, "seedmanagement.gardener.cloud/seed-secret-name", seedSecretName)
			metav1.SetMetaDataAnnotation(&managedSeed.ObjectMeta, "seedmanagement.gardener.cloud/seed-secret-namespace", gardenNamespaceGarden.Name)
			Expect(testClient.Patch(ctx, managedSeed, patch)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: seedSecretName, Namespace: gardenNamespaceGarden.Name}, &corev1.Secret{})).Should(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
				g.Expect(managedSeed.Annotations).NotTo(And(
					HaveKey("seedmanagement.gardener.cloud/seed-secret-name"),
					HaveKey("seedmanagement.gardener.cloud/seed-secret-namespace"),
				))
			}).Should(Succeed())
		})
	})

	Context("deletion", func() {
		JustBeforeEach(func() {
			reconcileShoot()
			checkIfSeedSecretsCreated()
			checkIfGardenletWasDeployed()
		})

		It("should remove the managed seed object and gardenlet deployment", func() {
			By("Mark ManagedSeed for deletion")
			Expect(testClient.Delete(ctx, managedSeed)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: backupSecretName, Namespace: gardenNamespaceGarden.Name}, &corev1.Secret{})).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: seedSecretName, Namespace: gardenNamespaceGarden.Name}, &corev1.Secret{})).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet", Namespace: gardenNamespaceShoot}, &appsv1.Deployment{})).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: gardenNamespaceShoot}, &corev1.Namespace{})).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(BeNotFoundError())
			}).Should(Succeed())
		})
	})
})

func bootstrapPtr(v seedmanagementv1alpha1.Bootstrap) *seedmanagementv1alpha1.Bootstrap { return &v }

func pullPolicyPtr(v corev1.PullPolicy) *corev1.PullPolicy { return &v }
