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

package shootvalidator_test

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var _ = Describe("ShootValidator tests", func() {
	var (
		seed              *gardencorev1beta1.Seed
		shoot             *gardencorev1beta1.Shoot
		testSecret        *corev1.Secret
		testSecretBinding *gardencorev1beta1.SecretBinding

		clusterRole *rbacv1.ClusterRole
		roleBinding *rbacv1.RoleBinding

		user           *envtest.AuthenticatedUser
		userTestClient client.Client
		userName       string

		err error
	)

	BeforeEach(func() {
		By("creating SecretBinding")
		testSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
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
				Namespace:    testNamespace.Name,
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
		log.Info("Created SecretBinding for test", "secretBinding", client.ObjectKeyFromObject(testSecretBinding))

		DeferCleanup(func() {
			By("deleting SecretBinding")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, testSecretBinding))).To(Succeed())
		})

		By("creating Seed")
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
			},
			Spec: gardencorev1beta1.SeedSpec{
				Provider: gardencorev1beta1.SeedProvider{
					Region: "region",
					Type:   "providerType",
				},
				Settings: &gardencorev1beta1.SeedSettings{
					ShootDNS:   &gardencorev1beta1.SeedSettingShootDNS{Enabled: true},
					Scheduling: &gardencorev1beta1.SeedSettingScheduling{Visible: true},
				},
				Networks: gardencorev1beta1.SeedNetworks{
					Pods:     "10.0.0.0/16",
					Services: "10.1.0.0/16",
					Nodes:    pointer.String("10.2.0.0/16"),
					ShootDefaults: &gardencorev1beta1.ShootNetworks{
						Pods:     pointer.String("100.128.0.0/11"),
						Services: pointer.String("100.72.0.0/13"),
					},
				},
				DNS: gardencorev1beta1.SeedDNS{
					IngressDomain: pointer.String("someingress.example.com"),
				},
			},
		}
		Expect(testClient.Create(ctx, seed)).To(Succeed())
		log.Info("Created Seed for test", "seed", client.ObjectKeyFromObject(seed))

		DeferCleanup(func() {
			By("deleting Seed")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, seed))).To(Succeed())
		})

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
			},
			Spec: gardencorev1beta1.ShootSpec{
				CloudProfileName:  cloudProfile.Name,
				SecretBindingName: testSecretBinding.Name,
				Region:            "region",
				Provider: gardencorev1beta1.Provider{
					Type: "providerType",
					Workers: []gardencorev1beta1.Worker{
						{
							Name:    "cpu-worker",
							Minimum: 2,
							Maximum: 2,
							Machine: gardencorev1beta1.Machine{Type: "large"},
						},
					},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{Version: "1.21.1"},
				Networking: gardencorev1beta1.Networking{Type: "foo-networking"},
			},
		}
	})

	JustBeforeEach(func() {
		Expect(testClient.Create(ctx, clusterRole)).To(Succeed())
		DeferCleanup(func() {
			By("Delete ClusterRole")
			Expect(testClient.Delete(ctx, clusterRole)).To(Or(Succeed(), BeNotFoundError()))
		})

		Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
		DeferCleanup(func() {
			By("Delete RoleBinding")
			Expect(testClient.Delete(ctx, roleBinding)).To(Or(Succeed(), BeNotFoundError()))
		})
	})

	Context("User without RBAC for shoots/binding", func() {
		BeforeEach(func() {
			clusterRole = &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "project-member",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"core.gardener.cloud"},
						Resources: []string{
							"shoots",
						},
						Verbs: []string{
							"create",
							"delete",
							"get",
						},
					},
				},
			}

			roleBinding = &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "project-member",
					Namespace: testNamespace.Name,
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     clusterRole.Name,
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "Group",
						Name:     "project:member",
					},
				},
			}

			userName = "member"
			user, err = testEnv.AddUser(envtest.User{
				Name:   userName,
				Groups: []string{"project:member"},
			}, &rest.Config{
				QPS:   1000.0,
				Burst: 2000.0,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(user).NotTo(BeNil())

			userTestClient, err = client.New(user.Config(), client.Options{Scheme: kubernetes.GardenScheme})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should be able to create a shoot without .spec.seedName succesfully", func() {
			By("creating Shoot")
			Expect(userTestClient.Create(ctx, shoot)).To(Succeed())
			log.Info("Created Shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

			DeferCleanup(func() {
				By("Delete Shoot")
				Expect(userTestClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
				}).Should(BeNotFoundError())
			})
		})

		It("should not be able to create a shoot with .spec.seedName", func() {
			By("creating Shoot")
			shoot.Spec.SeedName = &seed.Name
			err = userTestClient.Create(ctx, shoot)
			Expect(err).To(BeForbiddenError())
			Expect(err).To(MatchError(ContainSubstring("user %q is not allowed to set .spec.seedName", userName)))
		})
	})

	Context("User with RBAC for shoots/binding", func() {
		BeforeEach(func() {
			clusterRole = &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "project-admin",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"core.gardener.cloud"},
						Resources: []string{
							"shoots",
						},
						Verbs: []string{
							"create",
							"delete",
							"get",
						},
					},
					{
						APIGroups: []string{"core.gardener.cloud"},
						Resources: []string{
							"shoots/binding",
						},
						Verbs: []string{
							"update",
						},
					},
				},
			}

			roleBinding = &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "project-admin",
					Namespace: testNamespace.Name,
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     clusterRole.Name,
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "Group",
						Name:     "project:admin",
					},
				},
			}

			userName = "admin"
			user, err = testEnv.AddUser(envtest.User{
				Name:   userName,
				Groups: []string{"project:admin"},
			}, &rest.Config{
				QPS:   1000.0,
				Burst: 2000.0,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(user).NotTo(BeNil())

			userTestClient, err = client.New(user.Config(), client.Options{Scheme: kubernetes.GardenScheme})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should be able to create a shoot with .spec.seedName succesfully", func() {
			By("creating Shoot")
			shoot.Spec.SeedName = &seed.Name
			Expect(userTestClient.Create(ctx, shoot)).To(Succeed())
			log.Info("Created Shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

			DeferCleanup(func() {
				By("Delete Shoot")
				Expect(userTestClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
				}).Should(BeNotFoundError())
			})
		})
	})
})
