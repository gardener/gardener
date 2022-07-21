// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"context"
	"time"

	"github.com/gardener/gardener/pkg/api/indexer"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	bastioncontroller "github.com/gardener/gardener/pkg/controllermanager/controller/bastion"
	bastionstrategy "github.com/gardener/gardener/pkg/registry/operations/bastion"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ = Describe("Bastion controller tests", func() {
	var (
		fakeClock   clock.Clock
		maxLifeTime time.Duration

		resourceName string
		objectKey    client.ObjectKey

		shoot   *gardencorev1beta1.Shoot
		bastion *operationsv1alpha1.Bastion
	)

	BeforeEach(func() {
		fakeClock = clock.RealClock{}
		maxLifeTime = time.Hour

		resourceName = "test-" + utils.ComputeSHA256Hex([]byte(CurrentSpecReport().LeafNodeLocation.String()))[:8]
		objectKey = client.ObjectKey{Namespace: testNamespace.Name, Name: resourceName}

		providerType := "foo-provider"
		seedName := "foo"

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: kutil.ObjectMetaFromKey(objectKey),
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
				SeedName: &seedName,
			},
		}
		bastion = &operationsv1alpha1.Bastion{
			ObjectMeta: kutil.ObjectMetaFromKey(objectKey),
			Spec: operationsv1alpha1.BastionSpec{
				ShootRef: corev1.LocalObjectReference{
					Name: shoot.Name,
				},
				SeedName:     &seedName,
				ProviderType: &providerType,
				SSHPublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDcSZKq0lM9w+ElLp9I9jFvqEFbOV1+iOBX7WEe66GvPLOWl9ul03ecjhOf06+FhPsWFac1yaxo2xj+SJ+FVZ3DdSn4fjTpS9NGyQVPInSZveetRw0TV0rbYCFBTJuVqUFu6yPEgdcWq8dlUjLqnRNwlelHRcJeBfACBZDLNSxjj0oUz7ANRNCEne1ecySwuJUAz3IlNLPXFexRT0alV7Nl9hmJke3dD73nbeGbQtwvtu8GNFEoO4Eu3xOCKsLw6ILLo4FBiFcYQOZqvYZgCb4ncKM52bnABagG54upgBMZBRzOJvWp0ol+jK3Em7Vb6ufDTTVNiQY78U6BAlNZ8Xg+LUVeyk1C6vWjzAQf02eRvMdfnRCFvmwUpzbHWaVMsQm8gf3AgnTUuDR0ev1nQH/5892wZA86uLYW/wLiiSbvQsqtY1jSn9BAGFGdhXgWLAkGsd/E1vOT+vDcor6/6KjHBm0rG697A3TDBRkbXQ/1oFxcM9m17RteCaXuTiAYWMqGKDoJvTMDc4L+Uvy544pEfbOH39zfkIYE76WLAFPFsUWX6lXFjQrX3O7vEV73bCHoJnwzaNd03PSdJOw+LCzrTmxVezwli3F9wUDiBRB0HkQxIXQmncc1HSecCKALkogIK+1e1OumoWh6gPdkF4PlTMUxRitrwPWSaiUIlPfCpQ== you@example.com",
				Ingress: []operationsv1alpha1.BastionIngressPolicy{{
					IPBlock: networkingv1.IPBlock{CIDR: "1.2.3.4/32"},
				}},
			},
		}
	})

	JustBeforeEach(func() {
		By("setting up manager")
		mgr, err := manager.New(restConfig, manager.Options{
			Scheme:             kubernetes.GardenScheme,
			MetricsBindAddress: "0",
			Namespace:          testNamespace.Name,
		})
		Expect(err).NotTo(HaveOccurred())

		By("setting up field indexes")
		Expect(indexer.AddBastionShootName(ctx, mgr.GetFieldIndexer())).To(Succeed())

		By("registering controller")
		Expect((&bastioncontroller.Reconciler{
			Config: config.BastionControllerConfiguration{
				ConcurrentSyncs: pointer.Int(5),
				MaxLifetime:     &metav1.Duration{Duration: maxLifeTime},
			},
			Clock: fakeClock,
		}).AddToManager(mgr)).To(Succeed())

		By("starting manager")
		mgrContext, mgrCancel := context.WithCancel(ctx)

		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(mgrContext)).To(Succeed())
		}()

		DeferCleanup(func() {
			By("stopping manager")
			mgrCancel()
		})
	})

	JustBeforeEach(func() {
		if shoot != nil {
			By("Create Shoot")
			Expect(testClient.Create(ctx, shoot)).To(Succeed())
			log.Info("Created shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

			DeferCleanup(func() {
				By("Delete Shoot")
				Expect(client.IgnoreNotFound(gardener.ConfirmDeletion(ctx, testClient, shoot))).To(Succeed())
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot))).To(Succeed())
			})
		}

		By("Create Bastion")
		Expect(testClient.Create(ctx, bastion)).To(Succeed())
		log.Info("Created bastion for test", "bastion", client.ObjectKeyFromObject(bastion))

		DeferCleanup(func() {
			By("Delete Bastion")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, bastion))).To(Succeed())
		})
	})

	Context("shoot does not exist", func() {
		BeforeEach(func() {
			shoot = nil
		})

		It("should delete Bastion", func() {
			Eventually(func() error {
				return testClient.Get(ctx, objectKey, bastion)
			}).Should(BeNotFoundError())
		})
	})

	Context("shoot is in deletion", func() {
		JustBeforeEach(func() {
			// add finalizer to prolong shoot deletion
			By("Add finalizer to Shoot")
			patch := client.MergeFrom(shoot.DeepCopy())
			Expect(controllerutil.AddFinalizer(shoot, testID)).To(BeTrue())
			Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

			DeferCleanup(func() {
				By("Remove finalizer from Shoot")
				patch := client.MergeFrom(shoot.DeepCopy())
				Expect(controllerutil.RemoveFinalizer(shoot, testID)).To(BeTrue())
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())
			})

			By("Mark Shoot for deletion")
			Expect(gardener.ConfirmDeletion(ctx, testClient, shoot)).To(Succeed())
			Expect(testClient.Delete(ctx, shoot)).To(Succeed())
		})

		It("should delete Bastion", func() {
			Eventually(func() error {
				return testClient.Get(ctx, objectKey, bastion)
			}).Should(BeNotFoundError())
		})
	})

	Context("shoot has been migrated to another seed", func() {
		JustBeforeEach(func() {
			var err error

			By("Change Shoot's .spec.seedName")
			shoot.Spec.SeedName = pointer.String("another-seed")
			shoot, err = testCoreClient.CoreV1beta1().Shoots(shoot.GetNamespace()).UpdateBinding(ctx, shoot.GetName(), shoot, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete Bastion", func() {
			Eventually(func() error {
				return testClient.Get(ctx, objectKey, bastion)
			}).Should(BeNotFoundError())
		})
	})

	Context("shoot exists, is not in deletion and on the same seed", func() {
		Context("bastion is not yet expired", func() {
			It("should not delete Bastion", func() {
				Consistently(func() error {
					return testClient.Get(ctx, objectKey, bastion)
				}).Should(Succeed())
			})
		})

		Context("bastion is expired", func() {
			BeforeEach(func() {
				fakeClock = testclock.NewFakeClock(time.Now().Add(24 * time.Hour))
			})

			It("should delete Bastion", func() {
				Eventually(func() error {
					return testClient.Get(ctx, objectKey, bastion)
				}).Should(BeNotFoundError())
			})
		})

		Context("bastion will expire", func() {
			BeforeEach(func() {
				fakeClock = testclock.NewFakeClock(time.Now().Add(bastionstrategy.TimeToLive).Add(10 * time.Millisecond))
			})

			It("should requeue delete Bastion", func() {
				Eventually(func() error {
					return testClient.Get(ctx, objectKey, bastion)
				}).Should(BeNotFoundError())
			})
		})

		Context("bastion's max lifetime has been reached", func() {
			BeforeEach(func() {
				maxLifeTime = -time.Hour
			})

			It("should delete Bastion", func() {
				Eventually(func() error {
					return testClient.Get(ctx, objectKey, bastion)
				}).Should(BeNotFoundError())
			})
		})

		Context("bastion's max lifetime will be reached", func() {
			BeforeEach(func() {
				maxLifeTime = 10 * time.Millisecond
			})

			It("should requeue and delete Bastion", func() {
				Eventually(func() error {
					return testClient.Get(ctx, objectKey, bastion)
				}).Should(BeNotFoundError())
			})
		})
	})
})
