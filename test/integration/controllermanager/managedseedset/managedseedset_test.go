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

package managedseedset_test

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("ManagedSeedSet controller test", func() {
	var (
		shoot           *gardencorev1beta1.Shoot
		seed            *gardencorev1beta1.Seed
		managedSeed     *seedmanagementv1alpha1.ManagedSeed
		managedSeedSet  *seedmanagementv1alpha1.ManagedSeedSet
		selector        labels.Selector
		err             error
		shootList       *gardencorev1beta1.ShootList
		seedList        *gardencorev1beta1.SeedList
		managedSeedList *seedmanagementv1alpha1.ManagedSeedList
	)

	BeforeEach(func() {
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{testID: testRunID},
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

		managedSeed = &seedmanagementv1alpha1.ManagedSeed{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: gardenNamespace.Name,
				Labels:    map[string]string{testID: testRunID},
			},
			Spec: seedmanagementv1alpha1.ManagedSeedSpec{
				SeedTemplate: &gardencorev1beta1.SeedTemplate{
					ObjectMeta: seed.ObjectMeta,
					Spec:       seed.Spec,
				},
			},
		}

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: gardenNamespace.Name,
				Labels: map[string]string{
					testID:                       testRunID,
					v1beta1constants.ShootStatus: "healthy",
				},
			},
			Spec: gardencorev1beta1.ShootSpec{
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
				Region:            "some-region",
				SecretBindingName: "shoot-operator-foo",
			},
		}

		managedSeedSet = &seedmanagementv1alpha1.ManagedSeedSet{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
				Namespace:    gardenNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: seedmanagementv1alpha1.ManagedSeedSetSpec{
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{testID: testRunID},
				},
				Template: seedmanagementv1alpha1.ManagedSeedTemplate{
					ObjectMeta: managedSeed.ObjectMeta,
					Spec:       managedSeed.Spec,
				},
				ShootTemplate: gardencorev1beta1.ShootTemplate{
					ObjectMeta: shoot.ObjectMeta,
					Spec:       shoot.Spec,
				},
				UpdateStrategy: &seedmanagementv1alpha1.UpdateStrategy{
					Type: updateStrategyTypePtr(seedmanagementv1alpha1.RollingUpdateStrategyType),
					RollingUpdate: &seedmanagementv1alpha1.RollingUpdateStrategy{
						Partition: pointer.Int32(0),
					},
				},
			},
		}

		shootList = &gardencorev1beta1.ShootList{}
		seedList = &gardencorev1beta1.SeedList{}
		managedSeedList = &seedmanagementv1alpha1.ManagedSeedList{}
		selector, err = metav1.LabelSelectorAsSelector(&managedSeedSet.Spec.Selector)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		By("Create ManagedSeedSet")
		Expect(testClient.Create(ctx, managedSeedSet)).To(Succeed())
		log.Info("Created ManagedSeedSet for test", "managedSeedSet", client.ObjectKeyFromObject(managedSeedSet))

		DeferCleanup(func() {
			By("Delete ManagedSeedSet")
			Expect(testClient.Delete(ctx, managedSeedSet)).To(Or(Succeed(), BeNotFoundError()))
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), managedSeedSet)
			}).Should(BeNotFoundError())
		})
	})

	Context("reconcile", func() {
		It("should add the finalizer", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), managedSeedSet)).To(Succeed())
				g.Expect(managedSeedSet.Finalizers).To(ConsistOf("gardener"))
			}).Should(Succeed())
		})

		It("should create Shoot from shoot template and set the status.replica value to 1 (default value)", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), managedSeedSet)).To(Succeed())
				g.Expect(managedSeedSet.Status.Replicas).To(Equal(int32(1)))
				g.Expect(testClient.List(ctx, shootList, client.InNamespace(managedSeedSet.Namespace), client.MatchingLabelsSelector{Selector: selector})).To(Succeed())
				g.Expect(shootList.Items).To(HaveLen(1))
			}).Should(Succeed())
		})

		It("should create ManagedSeed when shoot is reconciled successfully", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.List(ctx, shootList, client.InNamespace(managedSeedSet.Namespace), client.MatchingLabelsSelector{Selector: selector})).To(Succeed())
				g.Expect(shootList.Items).To(HaveLen(1))
			}).Should(Succeed())

			By("Mark the Shoot as 'successfully created'")
			patch := client.MergeFrom(shootList.Items[0].DeepCopy())
			shootList.Items[0].Status = gardencorev1beta1.ShootStatus{
				ObservedGeneration: shootList.Items[0].GetGeneration(),
				LastOperation: &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeCreate,
					State: gardencorev1beta1.LastOperationStateSucceeded,
				},
			}
			Expect(testClient.Status().Patch(ctx, &shootList.Items[0], patch)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.List(ctx, managedSeedList, client.InNamespace(managedSeedSet.Namespace), client.MatchingLabelsSelector{Selector: selector})).To(Succeed())
				g.Expect(managedSeedList.Items).To(HaveLen(1))
			}).Should(Succeed())
		})

		It("should mark the replica as ready when Shoot is healthy and successfully created, ManagedSeed has SeedRegistered condition, and Seed ready conditions are satisfied", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.List(ctx, shootList, client.InNamespace(managedSeedSet.Namespace), client.MatchingLabelsSelector{Selector: selector})).To(Succeed())
				g.Expect(shootList.Items).To(HaveLen(1))
			}).Should(Succeed())

			By("Mark the Shoot as 'successfully created'")
			patch := client.MergeFrom(shootList.Items[0].DeepCopy())
			shootList.Items[0].Status = gardencorev1beta1.ShootStatus{
				ObservedGeneration: shootList.Items[0].GetGeneration(),
				LastOperation: &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeCreate,
					State: gardencorev1beta1.LastOperationStateSucceeded,
				},
			}
			Expect(testClient.Status().Patch(ctx, &shootList.Items[0], patch)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.List(ctx, managedSeedList, client.InNamespace(managedSeedSet.Namespace), client.MatchingLabelsSelector{Selector: selector})).To(Succeed())
				g.Expect(managedSeedList.Items).To(HaveLen(1))
			}).Should(Succeed())

			By("Mark the ManagedSeed condition as SeedRegistered")
			patch = client.MergeFrom(managedSeedList.Items[0].DeepCopy())
			managedSeedList.Items[0].Status = seedmanagementv1alpha1.ManagedSeedStatus{
				ObservedGeneration: managedSeedList.Items[0].GetGeneration(),
				Conditions: []gardencorev1beta1.Condition{
					{Type: seedmanagementv1alpha1.ManagedSeedSeedRegistered, Status: gardencorev1beta1.ConditionTrue},
				},
			}
			Expect(testClient.Status().Patch(ctx, &managedSeedList.Items[0], patch)).To(Succeed())

			By("Create Seed manually as ManagedSeed controller is not running in the test")
			seed.Name = managedSeedList.Items[0].Name
			Expect(testClient.Create(ctx, seed)).To(Succeed())
			log.Info("Created Seed for ManagedSeed", "seed", client.ObjectKeyFromObject(seed), "managedSeed", client.ObjectKeyFromObject(&managedSeedList.Items[0]))

			DeferCleanup(func() {
				By("Delete Seed")
				Expect(testClient.Delete(ctx, seed)).To(Or(Succeed(), BeNotFoundError()))
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)
				}).Should(BeNotFoundError())
			})

			Eventually(func(g Gomega) {
				g.Expect(testClient.List(ctx, seedList, client.MatchingLabelsSelector{Selector: selector})).To(Succeed())
				g.Expect(seedList.Items).To(HaveLen(1))
			}).Should(Succeed())

			By("Mark the Seed as Ready")
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
			patch = client.MergeFrom(seed.DeepCopy())
			seed.Status = gardencorev1beta1.SeedStatus{
				ObservedGeneration: seed.GetGeneration(),
				Conditions: []gardencorev1beta1.Condition{
					{Type: gardencorev1beta1.SeedGardenletReady, Status: gardencorev1beta1.ConditionTrue},
					{Type: gardencorev1beta1.SeedBootstrapped, Status: gardencorev1beta1.ConditionTrue},
					{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					{Type: gardencorev1beta1.SeedBackupBucketsReady, Status: gardencorev1beta1.ConditionTrue},
				},
			}
			Expect(testClient.Status().Patch(ctx, seed, patch)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), managedSeedSet)).To(Succeed())
				g.Expect(managedSeedSet.Status.PendingReplica).To(BeNil())
				g.Expect(managedSeedSet.Status.ReadyReplicas).To(Equal(int32(1)))
			}).Should(Succeed())
		})
	})

	Context("scale-out", func() {
		It("should create new replica and update the status.replicas field because spec.replicas value was increased", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), managedSeedSet)).To(Succeed())
				g.Expect(managedSeedSet.Status.Replicas).To(Equal(int32(1)))
			}).Should(Succeed())

			By("Update the Replicas to 2")
			patch := client.MergeFrom(managedSeedSet.DeepCopy())
			managedSeedSet.Spec.Replicas = pointer.Int32(2)
			Expect(testClient.Patch(ctx, managedSeedSet, patch)).To(Succeed())

			By("Mark the pending replica to nil, to enable controller to pick the next replica")
			patch = client.MergeFrom(managedSeedSet.DeepCopy())
			managedSeedSet.Status.PendingReplica = nil
			Expect(testClient.Status().Patch(ctx, managedSeedSet, patch)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), managedSeedSet)).To(Succeed())
				g.Expect(managedSeedSet.Status.Replicas).To(Equal(int32(2)))
				g.Expect(testClient.List(ctx, shootList, client.InNamespace(managedSeedSet.Namespace), client.MatchingLabelsSelector{Selector: selector})).To(Succeed())
				g.Expect(shootList.Items).To(HaveLen(2))
			}).Should(Succeed())
		})
	})

	Context("scale-in", func() {
		BeforeEach(func() {
			managedSeedSet.Spec.Replicas = pointer.Int32(2)
		})

		It("should delete replicas and update the status.replicas field because spec.replicas value was updated with lower value", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), managedSeedSet)).To(Succeed())
				patch := client.MergeFrom(managedSeedSet.DeepCopy())
				managedSeedSet.Status.PendingReplica = nil
				Expect(testClient.Status().Patch(ctx, managedSeedSet, patch)).To(Succeed())
				g.Expect(managedSeedSet.Status.Replicas).To(Equal(int32(2)))
			}).Should(Succeed())

			By("Update the Replicas to 1")
			patch := client.MergeFrom(managedSeedSet.DeepCopy())
			managedSeedSet.Spec.Replicas = pointer.Int32(1)
			Expect(testClient.Patch(ctx, managedSeedSet, patch)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), managedSeedSet)).To(Succeed())
				g.Expect(managedSeedSet.Status.Replicas).To(Equal(int32(1)))
			}).Should(Succeed())
		})
	})

	Context("deletion timestamp set", func() {
		JustBeforeEach(func() {
			// add finalizer to prolong managedSeedSet deletion
			By("Add finalizer to ManagedSeedSet")
			patch := client.MergeFrom(managedSeedSet.DeepCopy())
			Expect(controllerutil.AddFinalizer(managedSeedSet, testID)).To(BeTrue())
			Expect(testClient.Patch(ctx, managedSeedSet, patch)).To(Succeed())

			DeferCleanup(func() {
				By("Remove finalizer from ManagedSeedSet")
				patch := client.MergeFrom(managedSeedSet.DeepCopy())
				Expect(controllerutil.RemoveFinalizer(managedSeedSet, testID)).To(BeTrue())
				Expect(testClient.Patch(ctx, managedSeedSet, patch)).To(Succeed())
			})
		})

		It("should remove the gardener finalizer when deletion timestamp is set", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), managedSeedSet)).To(Succeed())
				g.Expect(managedSeedSet.Finalizers).To(ContainElement("gardener"))
			}).Should(Succeed())

			By("Mark ManagedSeedSet for deletion")
			Expect(testClient.Delete(ctx, managedSeedSet)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), managedSeedSet)).To(Succeed())
				g.Expect(managedSeedSet.Finalizers).NotTo(ContainElement("gardener"))
			}).Should(Succeed())
		})
	})
})

func updateStrategyTypePtr(v seedmanagementv1alpha1.UpdateStrategyType) *seedmanagementv1alpha1.UpdateStrategyType {
	return &v
}
