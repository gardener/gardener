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

package managedseed_test

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("ManagedSeed controller test", func() {
	var (
		shoot             *gardencorev1beta1.Shoot
		managedSeed       *seedmanagementv1alpha1.ManagedSeed
		kubeconfigSecret  *corev1.Secret
		testSecretBinding *gardencorev1beta1.SecretBinding
		testSecret        *corev1.Secret

		reconcileShoot = func() {
			By("Patch the Shoot as Reconciled")
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.ObservedGeneration = shoot.Generation
			shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())
		}

		deployGardenlet = func() {
			secret := &corev1.Secret{}
			EventuallyWithOffset(1, func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "test-backup-secret", Namespace: gardenNamespace.Name}, secret)).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "test-seed-secret", Namespace: gardenNamespace.Name}, secret)).To(Succeed())
			}).Should(Succeed())

			EventuallyWithOffset(1, func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:apiserver-sni"}, &rbacv1.ClusterRole{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet"}, &rbacv1.ClusterRole{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:managed-istio"}, &rbacv1.ClusterRole{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:apiserver-sni"}, &rbacv1.ClusterRoleBinding{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet"}, &rbacv1.ClusterRoleBinding{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:managed-istio"}, &rbacv1.ClusterRoleBinding{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener-system-critical"}, &schedulingv1.PriorityClass{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet", Namespace: gardenNamespace.Name}, &rbacv1.Role{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet", Namespace: gardenNamespace.Name}, &rbacv1.RoleBinding{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet-kubeconfig-bootstrap", Namespace: gardenNamespace.Name}, &corev1.Secret{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet", Namespace: gardenNamespace.Name}, &corev1.Service{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet", Namespace: gardenNamespace.Name}, &corev1.ServiceAccount{})).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet", Namespace: gardenNamespace.Name}, &appsv1.Deployment{})).To(Succeed())
			}).Should(Succeed())
		}
	)

	BeforeEach(func() {
		gardenletConfig, err := encoding.EncodeGardenletConfiguration(&gardenletconfigv1alpha1.GardenletConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
				Kind:       "GardenletConfiguration",
			},
			GardenClientConnection: &gardenletconfigv1alpha1.GardenClientConnection{
				KubeconfigSecret: &corev1.SecretReference{
					Name:      "gardenlet-kubeconfig",
					Namespace: gardenNamespace.Name,
				},
			},
			SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{testID: testRunID},
						Annotations: map[string]string{
							"bar": "baz",
						},
					},
					Spec: gardencorev1beta1.SeedSpec{
						Backup: &gardencorev1beta1.SeedBackup{
							Provider: "test",
							Region:   pointer.String("bar"),
							SecretRef: corev1.SecretReference{
								Name:      "test-backup-secret",
								Namespace: gardenNamespace.Name,
							},
						},
						SecretRef: &corev1.SecretReference{
							Name:      "test-seed-secret",
							Namespace: gardenNamespace.Name,
						},
						DNS: gardencorev1beta1.SeedDNS{
							IngressDomain: pointer.String("someingress.example.com"),
						},
					},
				},
			},
		})
		Expect(err).To(Succeed())

		managedSeed = &seedmanagementv1alpha1.ManagedSeed{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "managedseed-",
				Namespace:    gardenNamespace.Name,
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
				Name:      "shoot" + testRunID,
				Namespace: gardenNamespace.Name,
				Labels:    map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName:         &seed.Name,
				CloudProfileName: "foo",
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.20.1",
				},
				Networking: gardencorev1beta1.Networking{
					Type: "foo",
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
		By("creating SecretBinding")
		testSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    gardenNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}
		Expect(testClient.Create(ctx, testSecret)).To(Succeed())
		log.Info("Created Secret for test", "secret", client.ObjectKeyFromObject(testSecret))

		DeferCleanup(func() {
			By("deleting Secret")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, testSecret))).To(Succeed())
		})

		testSecretBinding = &gardencorev1beta1.SecretBinding{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    gardenNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Provider: &gardencorev1beta1.SecretBindingProvider{
				Type: "providerType",
			},
			SecretRef: corev1.SecretReference{
				Name:      testSecret.Name,
				Namespace: testSecret.Namespace,
			},
		}
		Expect(testClient.Create(ctx, testSecretBinding)).To(Succeed())
		log.Info("Created SecretBinding for test", "secretbinding", client.ObjectKeyFromObject(testSecretBinding))

		DeferCleanup(func() {
			By("deleting SecretBinding")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, testSecretBinding))).To(Succeed())
		})

		By("Create Shoot")
		shoot.Spec.SecretBindingName = testSecretBinding.Name
		Expect(testClient.Create(ctx, shoot)).To(Succeed())
		log.Info("Created Shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

		DeferCleanup(func() {
			By("Delete Shoot")
			Expect(testClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Ensure Shoot is created")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
		}).Should(Succeed())

		By("Create ManagedSeed")
		managedSeed.Spec.Shoot = &seedmanagementv1alpha1.Shoot{Name: shoot.Name}
		Expect(testClient.Create(ctx, managedSeed)).To(Succeed())
		log.Info("Created ManagedSeed for test", "managedseed", client.ObjectKeyFromObject(managedSeed))

		DeferCleanup(func() {
			By("Delete ManagedSeed")
			Expect(testClient.Delete(ctx, managedSeed)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Create kubeconfig Secret")
		kubeconfigSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shoot.Name + ".kubeconfig",
				Namespace: shoot.Namespace,
				Labels:    map[string]string{testID: testRunID},
			},
			Data: map[string][]byte{
				"kubeconfig": []byte("kubeconfig"),
			},
		}
		Expect(testClient.Create(ctx, kubeconfigSecret)).To(Succeed())
		log.Info("Created kubeconfig Secret", "kubeconfigSecret", client.ObjectKeyFromObject(kubeconfigSecret))

		DeferCleanup(func() {
			By("Delete kubeconfig Secret")
			Expect(testClient.Delete(ctx, kubeconfigSecret)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("ensure finalizer is added")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
			g.Expect(managedSeed.Finalizers).To(ConsistOf("gardener"))
		}).Should(Succeed())
	})

	Context("reconciliation", func() {
		It("should set the ShootReconciled status to false because the shoot is not yet reconciled", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
				condition := gardencorev1beta1helper.GetCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedShootReconciled)
				g.Expect(condition).NotTo(BeNil())
				g.Expect(condition.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				g.Expect(condition.Reason).To(Equal(gardencorev1beta1.EventReconciling))
			}).Should(Succeed())
		})

		It("should set the ShootRecociled status to true when the shoot is reconciled successfully", func() {
			reconcileShoot()

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
				condition := gardencorev1beta1helper.GetCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedShootReconciled)
				g.Expect(condition).NotTo(BeNil())
				g.Expect(condition.Status).To(Equal(gardencorev1beta1.ConditionTrue))
				g.Expect(condition.Reason).To(Equal(gardencorev1beta1.EventReconciled))
			}).Should(Succeed())
		})

		Context("create or update seed secrets", func() {
			JustBeforeEach(func() {
				reconcileShoot()
			})

			It("should create secret specified in spec.secretRef field of seed template", func() {
				secret := &corev1.Secret{}
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "test-backup-secret", Namespace: gardenNamespace.Name}, secret)).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "test-seed-secret", Namespace: gardenNamespace.Name}, secret)).To(Succeed())
				}).Should(Succeed())
			})

			It("should deploy the gardenlet", func() {
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:apiserver-sni"}, &rbacv1.ClusterRole{})).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet"}, &rbacv1.ClusterRole{})).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:managed-istio"}, &rbacv1.ClusterRole{})).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:apiserver-sni"}, &rbacv1.ClusterRoleBinding{})).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet"}, &rbacv1.ClusterRoleBinding{})).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:managed-istio"}, &rbacv1.ClusterRoleBinding{})).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener-system-critical"}, &schedulingv1.PriorityClass{})).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet", Namespace: gardenNamespace.Name}, &rbacv1.Role{})).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet", Namespace: gardenNamespace.Name}, &rbacv1.RoleBinding{})).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet-kubeconfig-bootstrap", Namespace: gardenNamespace.Name}, &corev1.Secret{})).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet", Namespace: gardenNamespace.Name}, &corev1.Service{})).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet", Namespace: gardenNamespace.Name}, &corev1.ServiceAccount{})).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet", Namespace: gardenNamespace.Name}, &appsv1.Deployment{})).To(Succeed())
				}).Should(Succeed())
			})
		})
	})

	Context("deletion", func() {
		JustBeforeEach(func() {
			reconcileShoot()
			deployGardenlet()
		})

		It("should remove the managed seed object and gardenlet deployment", func() {
			By("Mark ManagedSeed for deletion")
			Expect(testClient.Delete(ctx, managedSeed)).To(Succeed())

			EventuallyWithOffset(1, func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet", Namespace: gardenNamespace.Name}, &appsv1.Deployment{})).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(BeNotFoundError())
			}).Should(Succeed())
		})
	})

})

func bootstrapPtr(v seedmanagementv1alpha1.Bootstrap) *seedmanagementv1alpha1.Bootstrap { return &v }

func pullPolicyPtr(v corev1.PullPolicy) *corev1.PullPolicy { return &v }
