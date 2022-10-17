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

package bastion_test

import (
	"fmt"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	bastioncontroller "github.com/gardener/gardener/pkg/gardenlet/controller/bastion"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("Bastion controller tests", func() {
	var (
		seed       *gardencorev1beta1.Seed
		shoot      *gardencorev1beta1.Shoot
		bastion    *operationsv1alpha1.Bastion
		extBastion *extensionsv1alpha1.Bastion

		bastionReady = func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(bastion), bastion)).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extBastion), extBastion)).To(Succeed())
				g.Expect(extBastion.Spec.Type).To(Equal(*bastion.Spec.ProviderType))
				g.Expect(extBastion.Spec.UserData).To(Equal(createUserData(bastion)))
				g.Expect(extBastion.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile))
			}).Should(Succeed())

			By("Patch the extension Bastion to satisfy the condition for readyness as there is no extension controller running in test")
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extBastion), extBastion)).To(Succeed())
			patch := client.MergeFrom(extBastion.DeepCopy())
			delete(extBastion.Annotations, v1beta1constants.GardenerOperation)
			Expect(testClient.Patch(ctx, extBastion, patch)).To(Succeed())

			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extBastion), extBastion)).To(Succeed())
			patch = client.MergeFrom(extBastion.DeepCopy())
			extBastion.Status = extensionsv1alpha1.BastionStatus{
				DefaultStatus: extensionsv1alpha1.DefaultStatus{
					ObservedGeneration: extBastion.Generation,
					LastOperation: &gardencorev1beta1.LastOperation{
						LastUpdateTime: metav1.Now(),
						State:          gardencorev1beta1.LastOperationStateSucceeded,
					},
				},
				Ingress: &corev1.LoadBalancerIngress{},
			}
			Expect(testClient.Status().Patch(ctx, extBastion, patch)).To(Succeed())
		}
	)

	BeforeEach(func() {
		defer test.WithVars(
			bastioncontroller.DefaultTimeout, 1500*time.Millisecond,
			bastioncontroller.DefaultInterval, 10*time.Millisecond,
			bastioncontroller.DefaultSevereThreshold, 900*time.Millisecond,
		)

		fakeClock.SetTime(time.Now())

		providerType := "foo-provider"

		By("creating seed")
		seed = &gardencorev1beta1.Seed{
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
					IngressDomain: pointer.String("someingress.example.com"),
				},
			},
		}
		Expect(testClient.Create(ctx, seed)).To(Succeed())
		log.Info("Created Seed for test", "seed", seed.Name)

		DeferCleanup(func() {
			By("deleting seed")
			Expect(testClient.Delete(ctx, seed)).To(Or(Succeed(), BeNotFoundError()))
		})

		patch := client.MergeFrom(seed.DeepCopy())
		seed.Status.ClusterIdentity = pointer.String(seedClusterIdentity)
		Expect(testClient.Status().Patch(ctx, seed, patch)).To(Succeed())

		By("creating shoot")
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "shoot-",
				Namespace:    gardenNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: "my-provider-account",
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
					Version: "1.21.1",
				},
				Networking: gardencorev1beta1.Networking{
					Type: "foo-networking",
				},
				SeedName: &seed.Name,
			},
		}
		Expect(testClient.Create(ctx, shoot)).To(Succeed())
		log.Info("Created shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

		DeferCleanup(func() {
			By("Delete Shoot")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot))).To(Succeed())
		})

		By("Patch the shoot status with TechincalID")
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
		patch = client.MergeFrom(shoot.DeepCopy())
		shoot.Status.TechnicalID = seedNamespace.Name
		Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

		bastion = &operationsv1alpha1.Bastion{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "bastion-",
				Namespace:    gardenNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: operationsv1alpha1.BastionSpec{
				ShootRef: corev1.LocalObjectReference{
					Name: shoot.Name,
				},
				SeedName:     &seed.Name,
				ProviderType: &providerType,
				SSHPublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDcSZKq0lM9w+ElLp9I9jFvqEFbOV1+iOBX7WEe66GvPLOWl9ul03ecjhOf06+FhPsWFac1yaxo2xj+SJ+FVZ3DdSn4fjTpS9NGyQVPInSZveetRw0TV0rbYCFBTJuVqUFu6yPEgdcWq8dlUjLqnRNwlelHRcJeBfACBZDLNSxjj0oUz7ANRNCEne1ecySwuJUAz3IlNLPXFexRT0alV7Nl9hmJke3dD73nbeGbQtwvtu8GNFEoO4Eu3xOCKsLw6ILLo4FBiFcYQOZqvYZgCb4ncKM52bnABagG54upgBMZBRzOJvWp0ol+jK3Em7Vb6ufDTTVNiQY78U6BAlNZ8Xg+LUVeyk1C6vWjzAQf02eRvMdfnRCFvmwUpzbHWaVMsQm8gf3AgnTUuDR0ev1nQH/5892wZA86uLYW/wLiiSbvQsqtY1jSn9BAGFGdhXgWLAkGsd/E1vOT+vDcor6/6KjHBm0rG697A3TDBRkbXQ/1oFxcM9m17RteCaXuTiAYWMqGKDoJvTMDc4L+Uvy544pEfbOH39zfkIYE76WLAFPFsUWX6lXFjQrX3O7vEV73bCHoJnwzaNd03PSdJOw+LCzrTmxVezwli3F9wUDiBRB0HkQxIXQmncc1HSecCKALkogIK+1e1OumoWh6gPdkF4PlTMUxRitrwPWSaiUIlPfCpQ== you@example.com",
				Ingress: []operationsv1alpha1.BastionIngressPolicy{{
					IPBlock: networkingv1.IPBlock{CIDR: "1.2.3.4/32"},
				}},
			},
		}
		Expect(testClient.Create(ctx, bastion)).To(Succeed())
		log.Info("Created bastion for test", "bastion", client.ObjectKeyFromObject(bastion))

		DeferCleanup(func() {
			By("Delete Bastion")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, bastion))).To(Succeed())
		})

		extBastion = &extensionsv1alpha1.Bastion{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bastion.Name,
				Namespace: shoot.Status.TechnicalID,
			},
		}
	})

	It("should add the finalizer", func() {
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(bastion), bastion)).To(Succeed())
			g.Expect(bastion.Finalizers).To(ConsistOf("gardener"))
		}).Should(Succeed())
	})

	Context("#Reconcile", func() {
		It("should create or patch the bastion in the seed cluster", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extBastion), extBastion)).To(Succeed())
				annotations := extBastion.GetAnnotations()
				g.Expect(annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile))
				g.Expect(annotations).To(HaveKey(v1beta1constants.GardenerTimestamp))
				g.Expect(extBastion.Spec.Type).To(Equal(*bastion.Spec.ProviderType))
				g.Expect(extBastion.Spec.UserData).To(Equal(createUserData(bastion)))
			}).Should(Succeed())
		})

		It("should set BastionReady to True once extension Bastion is ready", func() {
			bastionReady()

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extBastion), extBastion)).To(Succeed())
				g.Expect(extBastion.Status.ObservedGeneration).To(Equal(int64(1)))
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(bastion), bastion)).To(Succeed())
				g.Expect(bastion.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(operationsv1alpha1.BastionReady),
					"Status":  Equal(gardencorev1alpha1.ConditionTrue),
					"Reason":  Equal("SuccessfullyReconciled"),
					"Message": Equal("The bastion has been reconciled successfully."),
				})))
				g.Expect(bastion.Status.ObservedGeneration).To(Equal(pointer.Int64(bastion.Generation)))
			}).Should(Succeed())
		})
	})

	Context("#Delete", func() {
		JustBeforeEach(func() {
			bastionReady()

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extBastion), extBastion)).To(Succeed())
				g.Expect(extBastion.Status.ObservedGeneration).To(Equal(int64(1)))
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(bastion), bastion)).To(Succeed())
				g.Expect(bastion.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(operationsv1alpha1.BastionReady),
					"Status":  Equal(gardencorev1alpha1.ConditionTrue),
					"Reason":  Equal("SuccessfullyReconciled"),
					"Message": Equal("The bastion has been reconciled successfully."),
				})))
				g.Expect(bastion.Status.ObservedGeneration).To(Equal(pointer.Int64(bastion.Generation)))
			}).Should(Succeed())

			By("Add finalizer to Bastion")
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(bastion), bastion)).To(Succeed())
			patch := client.MergeFrom(bastion.DeepCopy())
			Expect(controllerutil.AddFinalizer(bastion, testID)).To(BeTrue())
			Expect(testClient.Patch(ctx, bastion, patch)).To(Succeed())

			By("Mark Bastion for deletion")
			Expect(testClient.Delete(ctx, bastion)).To(Succeed())

			DeferCleanup(func() {
				By("Remove finalizer from Bastion")
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(bastion), bastion)).To(Succeed())
				patch := client.MergeFrom(bastion.DeepCopy())
				Expect(controllerutil.RemoveFinalizer(bastion, testID)).To(BeTrue())
				Expect(testClient.Patch(ctx, bastion, patch)).To(Succeed())
			})
		})

		It("should set the BastionReady as false and DeletionInProgress status", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(bastion), bastion)).To(Succeed())
				g.Expect(bastion.DeletionTimestamp).NotTo(BeNil())
				g.Expect(bastion.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(operationsv1alpha1.BastionReady),
					"Status":  Equal(gardencorev1alpha1.ConditionFalse),
					"Reason":  Equal("DeletionInProgress"),
					"Message": Equal("The bastion is being deleted."),
				})))
			}).Should(Succeed())
		})

		It("should delete the extension Bastion object", func() {
			extBastion := &extensionsv1alpha1.Bastion{}
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extBastion), extBastion)).To(BeNotFoundError())
			})
		})

		It("should remove the gardener finalizer", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(bastion), bastion)).To(Succeed())
				g.Expect(bastion.Finalizers).NotTo(ContainElement("gardener"))
			})
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
