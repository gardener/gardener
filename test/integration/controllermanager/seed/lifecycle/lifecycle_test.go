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

package lifecycle_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Seed Lifecycle controller tests", func() {
	var (
		seed            *gardencorev1beta1.Seed
		lease           *coordinationv1.Lease
		gardenNamespace *corev1.Namespace
		managedSeed     *seedmanagementv1alpha1.ManagedSeed
		shoot           *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		fakeClock.SetTime(time.Now())

		lease = &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
		}

		By("Create Lease")
		Expect(testClient.Create(ctx, lease)).To(Succeed())
		log.Info("Created Lease", "lease", client.ObjectKeyFromObject(lease))

		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, lease)).To(Or(Succeed(), BeNotFoundError()))
		})

		gardenNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "garden",
			},
		}
	})

	JustBeforeEach(func() {
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name:   lease.Name,
				Labels: map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.SeedSpec{
				Provider: gardencorev1beta1.SeedProvider{
					Region: "region",
					Type:   "providerType",
				},
				Ingress: &gardencorev1beta1.Ingress{
					Domain: "someingress.example.com",
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
					Nodes:    pointer.String("10.2.0.0/16"),
					ShootDefaults: &gardencorev1beta1.ShootNetworks{
						Pods:     pointer.String("100.128.0.0/11"),
						Services: pointer.String("100.72.0.0/13"),
					},
				},
			},
		}

		By("Create Seed")
		Expect(testClient.Create(ctx, seed)).To(Succeed())
		log.Info("Created Seed", "seed", client.ObjectKeyFromObject(seed))

		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, seed)).To(Succeed())
		})

		managedSeed = &seedmanagementv1alpha1.ManagedSeed{
			ObjectMeta: metav1.ObjectMeta{
				Name:      seed.Name,
				Namespace: gardenNamespace.Name,
				Labels:    map[string]string{testID: testRunID},
			},
			Spec: seedmanagementv1alpha1.ManagedSeedSpec{
				Shoot:     &seedmanagementv1alpha1.Shoot{Name: "foo"},
				Gardenlet: &seedmanagementv1alpha1.Gardenlet{},
			},
		}

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    managedSeed.Namespace,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: pointer.String("my-provider-account"),
				CloudProfileName:  "cloudprofile1",
				SeedName:          &seed.Name,
				Region:            "europe-central-1",
				Provider: gardencorev1beta1.Provider{
					Type: "foo-provider",
					Workers: []gardencorev1beta1.Worker{
						{
							Name:    "cpu-worker",
							Minimum: 3,
							Maximum: 3,
							Machine: gardencorev1beta1.Machine{
								Type: "large",
							},
						},
					},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.25.1",
				},
				Networking: &gardencorev1beta1.Networking{
					Type: pointer.String("foo-networking"),
				},
			},
		}
	})

	Context("when there is no GardenletReady condition", func() {
		It("should not change the GardenletReady condition", func() {
			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				g.Expect(seed.Status.Conditions).To(BeEmpty())
			}).Should(Succeed())
		})
	})

	Context("when there is a GardenletReady condition", func() {
		JustBeforeEach(func() {
			By("Add GardenletReady condition to Seed")
			patch := client.MergeFrom(seed.DeepCopy())
			seed.Status.Conditions = []gardencorev1beta1.Condition{{
				Type:               gardencorev1beta1.SeedGardenletReady,
				Status:             gardencorev1beta1.ConditionTrue,
				LastTransitionTime: metav1.Time{Time: fakeClock.Now().Add(-24 * time.Hour)},
			}}
			Expect(testClient.Status().Patch(ctx, seed, patch)).To(Succeed())
		})

		Context("when Lease object does not exist", func() {
			BeforeEach(func() {
				Expect(testClient.Delete(ctx, lease)).To(Succeed())
			})

			It("should change the condition to Unknown", func() {
				Eventually(func(g Gomega) gardencorev1beta1.ConditionStatus {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
					return seed.Status.Conditions[0].Status
				}).Should(Equal(gardencorev1beta1.ConditionUnknown))
			})
		})

		Context("when Lease object exists but is not maintained", func() {
			It("should change the condition to Unknown", func() {
				Eventually(func(g Gomega) gardencorev1beta1.ConditionStatus {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
					return seed.Status.Conditions[0].Status
				}).Should(Equal(gardencorev1beta1.ConditionUnknown))
			})
		})

		Context("when Lease object exists but was not renewed within grace period", func() {
			BeforeEach(func() {
				By("Update RenewTime of Lease")
				patch := client.MergeFrom(lease.DeepCopy())
				lease.Spec.RenewTime = microNow(fakeClock.Now().Add(-2 * seedMonitorPeriod))
				Expect(testClient.Patch(ctx, lease, patch)).To(Succeed())
			})

			It("should change the condition to Unknown", func() {
				Eventually(func(g Gomega) gardencorev1beta1.ConditionStatus {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
					return seed.Status.Conditions[0].Status
				}).Should(Equal(gardencorev1beta1.ConditionUnknown))
			})
		})

		Context("when Lease exists and is maintained and up-to-date", func() {
			BeforeEach(func() {
				By("Update RenewTime of Lease")
				patch := client.MergeFrom(lease.DeepCopy())
				lease.Spec.RenewTime = microNow(fakeClock.Now())
				Expect(testClient.Patch(ctx, lease, patch)).To(Succeed())
			})

			It("should not change the condition to Unknown", func() {
				Consistently(func(g Gomega) gardencorev1beta1.ConditionStatus {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
					return seed.Status.Conditions[0].Status
				}).Should(Equal(gardencorev1beta1.ConditionTrue))
			})
		})

		Context("rebootstrapping of ManagedSeed", func() {
			JustBeforeEach(func() {
				By("Create garden Namespace")
				Expect(testClient.Create(ctx, gardenNamespace)).To(Succeed())
				log.Info("Created garden Namespace", "namespace", client.ObjectKeyFromObject(gardenNamespace))

				By("Create ManagedSeed")
				Expect(testClient.Create(ctx, managedSeed)).To(Succeed())
				log.Info("Created ManagedSeed", "managedSeed", client.ObjectKeyFromObject(managedSeed))

				DeferCleanup(func() {
					Expect(testClient.Delete(ctx, managedSeed)).To(Succeed())
				})
			})

			It("should reconcile the ManagedSeed when client certificate is expired", func() {
				oldManagedSeedGeneration := managedSeed.Generation

				patch := client.MergeFrom(seed.DeepCopy())
				seed.Status.ClientCertificateExpirationTimestamp = &metav1.Time{Time: fakeClock.Now().Add(-time.Hour)}
				Expect(testClient.Status().Patch(ctx, seed, patch)).To(Succeed())

				Eventually(func(g Gomega) int64 {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
					return managedSeed.Generation
				}).Should(BeNumerically(">", oldManagedSeedGeneration))
			})
		})

		Context("changing Shoot status", func() {
			JustBeforeEach(func() {
				By("Create Shoot")
				Expect(testClient.Create(ctx, shoot)).To(Succeed())
				log.Info("Created Shoot", "shoot", client.ObjectKeyFromObject(shoot))

				By("Set shoot constraints and conditions to status True")
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Status = gardencorev1beta1.ShootStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.ShootAPIServerAvailable, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.ShootControlPlaneHealthy, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.ShootObservabilityComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.ShootEveryNodeReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.ShootSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
					Constraints: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.ShootHibernationPossible, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.ShootMaintenancePreconditionsSatisfied, Status: gardencorev1beta1.ConditionTrue},
					},
				}
				Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

				DeferCleanup(func() {
					Expect(testClient.Delete(ctx, shoot)).To(Succeed())
				})

				By("Update RenewTime of Lease")
				patch = client.MergeFrom(lease.DeepCopy())
				lease.Spec.RenewTime = microNow(fakeClock.Now().Add(-2 * seedMonitorPeriod))
				Expect(testClient.Patch(ctx, lease, patch)).To(Succeed())
			})

			It("should change the shoot conditions to Unknown only when shoot monitor period has passed", func() {
				Eventually(func(g Gomega) gardencorev1beta1.ConditionStatus {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
					return seed.Status.Conditions[0].Status
				}).Should(Equal(gardencorev1beta1.ConditionUnknown))

				Consistently(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					for _, constraint := range shoot.Status.Constraints {
						g.Expect(constraint.Status).To(Equal(gardencorev1beta1.ConditionTrue), "constraint "+string(constraint.Type)+" should have status True")
					}
					for _, condition := range shoot.Status.Conditions {
						g.Expect(condition.Status).To(Equal(gardencorev1beta1.ConditionTrue), "condition "+string(condition.Type)+" should have status True")
					}
				}).Should(Succeed())

				fakeClock.Step(2 * shootMonitorPeriod)

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					for _, constraint := range shoot.Status.Constraints {
						g.Expect(constraint.Status).To(Equal(gardencorev1beta1.ConditionUnknown), "constraint "+string(constraint.Type)+" should have status Unknown")
					}
					for _, condition := range shoot.Status.Conditions {
						g.Expect(condition.Status).To(Equal(gardencorev1beta1.ConditionUnknown), "condition "+string(condition.Type)+" should have status Unknown")
					}
				}).Should(Succeed())
			})
		})
	})
})

func microNow(t time.Time) *metav1.MicroTime {
	now := metav1.NewMicroTime(t)
	return &now
}
