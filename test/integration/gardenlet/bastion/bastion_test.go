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

package bastion_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Bastion controller tests", func() {
	var (
		seed              *gardencorev1beta1.Seed
		shoot             *gardencorev1beta1.Shoot
		operationsBastion *operationsv1alpha1.Bastion
		extensionsBastion *extensionsv1alpha1.Bastion
		cluster           *extensionsv1alpha1.Cluster
		seedNamespace     *corev1.Namespace

		reconcileExtensionsBastion = func() {
			EventuallyWithOffset(1, func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(operationsBastion), operationsBastion)).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionsBastion), extensionsBastion)).To(Succeed())
				g.Expect(extensionsBastion.Spec.Type).To(Equal(*operationsBastion.Spec.ProviderType))
				g.Expect(extensionsBastion.Spec.UserData).To(Equal(createUserData(operationsBastion)))
				g.Expect(extensionsBastion.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile))
			}).Should(Succeed())

			By("Patch the extension Bastion to satisfy the condition for readiness as there is no extension controller running in test")
			patch := client.MergeFrom(extensionsBastion.DeepCopy())
			delete(extensionsBastion.Annotations, v1beta1constants.GardenerOperation)
			ExpectWithOffset(1, testClient.Patch(ctx, extensionsBastion, patch)).To(Succeed())

			patch = client.MergeFrom(extensionsBastion.DeepCopy())
			extensionsBastion.Status = extensionsv1alpha1.BastionStatus{
				DefaultStatus: extensionsv1alpha1.DefaultStatus{
					ObservedGeneration: extensionsBastion.Generation,
					LastOperation: &gardencorev1beta1.LastOperation{
						LastUpdateTime: metav1.NewTime(fakeClock.Now()),
						State:          gardencorev1beta1.LastOperationStateSucceeded,
					},
				},
				Ingress: &corev1.LoadBalancerIngress{},
			}
			ExpectWithOffset(1, testClient.Status().Patch(ctx, extensionsBastion, patch)).To(Succeed())
		}
	)

	BeforeEach(func() {
		fakeClock.SetTime(time.Now())

		providerType := "foo-provider"

		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "seed-",
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.SeedSpec{
				Provider: gardencorev1beta1.SeedProvider{
					Region: "region",
					Type:   providerType,
				},
				Ingress: &gardencorev1beta1.Ingress{
					Domain: "seed.example.com",
					Controller: gardencorev1beta1.IngressController{
						Kind: "nginx",
					},
				},
				DNS: gardencorev1beta1.SeedDNS{
					Provider: &gardencorev1beta1.SeedDNSProvider{
						Type: providerType,
						SecretRef: corev1.SecretReference{
							Name:      "some-secret",
							Namespace: "some-namespace",
						},
					},
				},
				Networks: gardencorev1beta1.SeedNetworks{
					Pods:     "10.0.0.0/16",
					Services: "10.1.0.0/16",
					Nodes:    pointer.String("10.2.0.0/16"),
				},
			},
		}

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "shoot-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: pointer.String("my-provider-account"),
				CloudProfileName:  "test-cloudprofile",
				Region:            "foo-region",
				Provider: gardencorev1beta1.Provider{
					Type: providerType,
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
					Version: "1.22.1",
				},
				Networking: &gardencorev1beta1.Networking{
					Type: pointer.String("foo-networking"),
				},
			},
		}

		operationsBastion = &operationsv1alpha1.Bastion{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "bastion-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: operationsv1alpha1.BastionSpec{
				ProviderType: &providerType,
				SSHPublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDcSZKq0lM9w+ElLp9I9jFvqEFbOV1+iOBX7WEe66GvPLOWl9ul03ecjhOf06+FhPsWFac1yaxo2xj+SJ+FVZ3DdSn4fjTpS9NGyQVPInSZveetRw0TV0rbYCFBTJuVqUFu6yPEgdcWq8dlUjLqnRNwlelHRcJeBfACBZDLNSxjj0oUz7ANRNCEne1ecySwuJUAz3IlNLPXFexRT0alV7Nl9hmJke3dD73nbeGbQtwvtu8GNFEoO4Eu3xOCKsLw6ILLo4FBiFcYQOZqvYZgCb4ncKM52bnABagG54upgBMZBRzOJvWp0ol+jK3Em7Vb6ufDTTVNiQY78U6BAlNZ8Xg+LUVeyk1C6vWjzAQf02eRvMdfnRCFvmwUpzbHWaVMsQm8gf3AgnTUuDR0ev1nQH/5892wZA86uLYW/wLiiSbvQsqtY1jSn9BAGFGdhXgWLAkGsd/E1vOT+vDcor6/6KjHBm0rG697A3TDBRkbXQ/1oFxcM9m17RteCaXuTiAYWMqGKDoJvTMDc4L+Uvy544pEfbOH39zfkIYE76WLAFPFsUWX6lXFjQrX3O7vEV73bCHoJnwzaNd03PSdJOw+LCzrTmxVezwli3F9wUDiBRB0HkQxIXQmncc1HSecCKALkogIK+1e1OumoWh6gPdkF4PlTMUxRitrwPWSaiUIlPfCpQ== you@example.com",
				Ingress: []operationsv1alpha1.BastionIngressPolicy{{
					IPBlock: networkingv1.IPBlock{CIDR: "1.2.3.4/32"},
				}},
			},
		}
	})

	JustBeforeEach(func() {
		By("Create Seed")
		Expect(testClient.Create(ctx, seed)).To(Succeed())
		log.Info("Created Seed for test", "seed", seed.Name)

		DeferCleanup(func() {
			By("Delete Seed")
			Expect(testClient.Delete(ctx, seed)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Create Shoot")
		shoot.Spec.SeedName = &seed.Name
		Expect(testClient.Create(ctx, shoot)).To(Succeed())
		log.Info("Created Shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

		DeferCleanup(func() {
			By("Delete Shoot")
			Expect(testClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Wait until manager has observed shoot")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), &gardencorev1beta1.Shoot{})
		}).Should(Succeed())

		By("Patch the Shoot status with technical ID")
		patch := client.MergeFrom(shoot.DeepCopy())
		technicalID := gardenerutils.ComputeTechnicalID(project.Name, shoot)
		shoot.Status.TechnicalID = technicalID
		Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

		By("Create seed namespace for test")
		seedNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: technicalID,
			},
		}

		Expect(testClient.Create(ctx, seedNamespace)).To(Succeed())
		log.Info("Created shoot namespace in seed for test", "namespaceName", seedNamespace.Name)

		DeferCleanup(func() {
			By("Delete shoot namespace in seed")
			Expect(testClient.Delete(ctx, seedNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Create Cluster extension resource")
		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: technicalID,
			},
			Spec: extensionsv1alpha1.ClusterSpec{
				Shoot: runtime.RawExtension{
					Object: &gardencorev1beta1.Shoot{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "core.gardener.cloud/v1beta1",
							Kind:       "Shoot",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      shoot.Name,
							Namespace: shoot.Namespace,
						},
					},
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
		log.Info("Created Cluster for test", "cluster", client.ObjectKeyFromObject(cluster))

		DeferCleanup(func() {
			By("Delete Cluster")
			Expect(testClient.Delete(ctx, cluster)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Wait until manager has observed cluster creation")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)
		}).Should(Succeed())

		By("Create Bastion")
		operationsBastion.Spec.ShootRef = corev1.LocalObjectReference{Name: shoot.Name}
		operationsBastion.Spec.SeedName = &seed.Name
		Expect(testClient.Create(ctx, operationsBastion)).To(Succeed())
		log.Info("Created Bastion for test", "bastion", client.ObjectKeyFromObject(operationsBastion))

		extensionsBastion = &extensionsv1alpha1.Bastion{
			ObjectMeta: metav1.ObjectMeta{
				Name:      operationsBastion.Name,
				Namespace: seedNamespace.Name,
			},
		}

		DeferCleanup(func() {
			By("Delete Bastion")
			Expect(testClient.Delete(ctx, operationsBastion)).To(Or(Succeed(), BeNotFoundError()))

			By("Wait for Bastion to be gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(operationsBastion), operationsBastion)
			}).Should(BeNotFoundError())
		})

		By("Ensure finalizer is added to Bastion")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(operationsBastion), operationsBastion)).To(Succeed())
			g.Expect(operationsBastion.Finalizers).To(ConsistOf("gardener"))
		}).Should(Succeed())
	})

	Context("reconciliation", func() {
		It("should create or patch the Bastion in the Seed cluster", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionsBastion), extensionsBastion)).To(Succeed())
				g.Expect(extensionsBastion.GetAnnotations()).To(And(
					HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile),
					HaveKey(v1beta1constants.GardenerTimestamp),
				))
				g.Expect(extensionsBastion.Spec.Type).To(Equal(*operationsBastion.Spec.ProviderType))
				g.Expect(extensionsBastion.Spec.UserData).To(Equal(createUserData(operationsBastion)))
			}).Should(Succeed())
		})

		It("should set BastionReady to True once extension Bastion is ready", func() {
			reconcileExtensionsBastion()

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionsBastion), extensionsBastion)).To(Succeed())
				g.Expect(extensionsBastion.Status.ObservedGeneration).To(Equal((extensionsBastion.Generation)))
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(operationsBastion), operationsBastion)).To(Succeed())
				g.Expect(operationsBastion.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(operationsv1alpha1.BastionReady),
					"Status":  Equal(gardencorev1beta1.ConditionTrue),
					"Reason":  Equal("SuccessfullyReconciled"),
					"Message": Equal("The bastion has been reconciled successfully."),
				})))
				g.Expect(operationsBastion.Status.ObservedGeneration).To(Equal(pointer.Int64(operationsBastion.Generation)))
			}).Should(Succeed())
		})
	})

	Context("deletion", func() {
		It("should delete the extensions Bastions and the operations Bastion resource", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(operationsBastion), operationsBastion)).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionsBastion), extensionsBastion)).To(Succeed())
			}).Should(Succeed())

			By("Mark Bastion for deletion")
			Expect(testClient.Delete(ctx, operationsBastion)).To(Succeed())

			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(extensionsBastion), extensionsBastion)
			}).Should(BeNotFoundError())

			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(operationsBastion), operationsBastion)
			}).Should(BeNotFoundError())
		})
	})
})

func createUserData(bastion *operationsv1alpha1.Bastion) []byte {
	userData := fmt.Sprintf(`#!/bin/bash -eu

id gardener || useradd gardener -mU
mkdir -p /home/gardener/.ssh
echo "%s" > /home/gardener/.ssh/authorized_keys
chown gardener:gardener /home/gardener/.ssh/authorized_keys
echo "gardener ALL=(ALL) NOPASSWD:ALL" >/etc/sudoers.d/99-gardener-user
`, bastion.Spec.SSHPublicKey)

	return []byte(userData)
}
