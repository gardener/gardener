// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseedset_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ManagedSeedSet controller test", func() {
	var (
		shoot           *gardencorev1beta1.Shoot
		seed            *gardencorev1beta1.Seed
		gardenletConfig *gardenletconfigv1alpha1.GardenletConfiguration
		managedSeed     *seedmanagementv1alpha1.ManagedSeed
		managedSeedSet  *seedmanagementv1alpha1.ManagedSeedSet
		selector        labels.Selector
		shootList       *gardencorev1beta1.ShootList
		managedSeedList *seedmanagementv1alpha1.ManagedSeedList

		makeShootStateSucceeded = func() {
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
		}

		makeReplicaReady = func() {
			makeShootStateSucceeded()

			Eventually(func(g Gomega) {
				g.Expect(testClient.List(ctx, managedSeedList, client.InNamespace(managedSeedSet.Namespace), client.MatchingLabelsSelector{Selector: selector})).To(Succeed())
				g.Expect(managedSeedList.Items).To(HaveLen(1))
			}).Should(Succeed())

			By("Mark the ManagedSeed condition as SeedRegistered")
			patch := client.MergeFrom(managedSeedList.Items[0].DeepCopy())
			managedSeedList.Items[0].Status = seedmanagementv1alpha1.ManagedSeedStatus{
				ObservedGeneration: managedSeedList.Items[0].GetGeneration(),
				Conditions: []gardencorev1beta1.Condition{
					{Type: seedmanagementv1alpha1.SeedRegistered, Status: gardencorev1beta1.ConditionTrue},
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
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)
				}).Should(BeNotFoundError())
			})

			By("Mark the Seed as Ready")
			patch = client.MergeFrom(seed.DeepCopy())
			seed.Status = gardencorev1beta1.SeedStatus{
				ObservedGeneration: seed.GetGeneration(),
				Conditions: []gardencorev1beta1.Condition{
					{Type: gardencorev1beta1.SeedGardenletReady, Status: gardencorev1beta1.ConditionTrue},
					{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					{Type: gardencorev1beta1.SeedBackupBucketsReady, Status: gardencorev1beta1.ConditionTrue},
				},
			}
			Expect(testClient.Status().Patch(ctx, seed, patch)).To(Succeed())
		}
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
				Ingress: &gardencorev1beta1.Ingress{
					Domain: "seed.example.com",
					Controller: gardencorev1beta1.IngressController{
						Kind: "nginx",
					},
				},
				DNS: gardencorev1beta1.SeedDNS{
					Provider: &gardencorev1beta1.SeedDNSProvider{
						Type: "provider",
						SecretRef: corev1.SecretReference{
							Name:      "some-secret",
							Namespace: "some-namespace",
						},
					},
				},
				Networks: gardencorev1beta1.SeedNetworks{
					Pods:     "10.0.0.0/16",
					Services: "10.1.0.0/16",
					Nodes:    ptr.To("10.2.0.0/16"),
					ShootDefaults: &gardencorev1beta1.ShootNetworks{
						Pods:     ptr.To("100.128.0.0/11"),
						Services: ptr.To("100.72.0.0/13"),
					},
				},
			},
		}

		gardenletConfig = &gardenletconfigv1alpha1.GardenletConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
				Kind:       "GardenletConfiguration",
			},
			SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{
					ObjectMeta: seed.ObjectMeta,
					Spec:       seed.Spec,
				},
			},
		}

		gardenletConfigJson, err := encoding.EncodeGardenletConfigurationToBytes(gardenletConfig)
		utilruntime.Must(err)

		managedSeed = &seedmanagementv1alpha1.ManagedSeed{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: gardenNamespace.Name,
				Labels:    map[string]string{testID: testRunID},
			},
			Spec: seedmanagementv1alpha1.ManagedSeedSpec{
				Gardenlet: seedmanagementv1alpha1.GardenletConfig{
					Config: runtime.RawExtension{
						Raw: gardenletConfigJson,
					},
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
				CloudProfileName: ptr.To("foo"),
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.31.1",
				},
				Networking: &gardencorev1beta1.Networking{
					Type: ptr.To("foo"),
				},
				DNS: &gardencorev1beta1.DNS{
					Domain: ptr.To("replica-name.example.com"),
				},
				Provider: gardencorev1beta1.Provider{
					Type: "foo",
					Workers: []gardencorev1beta1.Worker{
						{
							Name: "some-worker",
							Machine: gardencorev1beta1.Machine{
								Type:         "some-machine-type",
								Architecture: ptr.To("amd64"),
							},
							Maximum: 2,
							Minimum: 1,
						},
					},
				},
				Region:            "some-region",
				SecretBindingName: ptr.To("shoot-operator-foo"),
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
					Type: ptr.To(seedmanagementv1alpha1.RollingUpdateStrategyType),
					RollingUpdate: &seedmanagementv1alpha1.RollingUpdateStrategy{
						Partition: ptr.To[int32](0),
					},
				},
			},
		}

		shootList = &gardencorev1beta1.ShootList{}
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
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), managedSeedSet)
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
			makeShootStateSucceeded()

			Eventually(func(g Gomega) {
				g.Expect(testClient.List(ctx, managedSeedList, client.InNamespace(managedSeedSet.Namespace), client.MatchingLabelsSelector{Selector: selector})).To(Succeed())
				g.Expect(managedSeedList.Items).To(HaveLen(1))

				managedSeed := managedSeedList.Items[0]
				g.Expect(managedSeed.Spec.Shoot.Name).To(Equal(shootList.Items[0].Name))
				g.Expect(managedSeed.Spec.Gardenlet).NotTo(BeNil())

				gardenletConfig, err := encoding.DecodeGardenletConfiguration(&managedSeed.Spec.Gardenlet.Config, true)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(gardenletConfig.SeedConfig.Spec.Provider.Type).To(Equal("providerType"))
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), managedSeedSet)).To(Succeed())
				g.Expect(managedSeedSet.Status.PendingReplica).NotTo(BeNil())
			}).Should(Succeed())
		})

		It("should mark the replica as ready when Shoot is healthy and successfully created, ManagedSeed has SeedRegistered condition, and Seed ready conditions are satisfied", func() {
			makeReplicaReady()

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
			managedSeedSet.Spec.Replicas = ptr.To[int32](2)
			Expect(testClient.Patch(ctx, managedSeedSet, patch)).To(Succeed())

			By("Make one replica ready, to enable controller to pick the next replica")
			makeReplicaReady()

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), managedSeedSet)).To(Succeed())
				g.Expect(managedSeedSet.Status.ReadyReplicas).To(Equal(int32(1)))
				g.Expect(managedSeedSet.Status.PendingReplica).NotTo(BeNil())
				g.Expect(managedSeedSet.Status.Replicas).To(Equal(int32(2)))
			}).Should(Succeed())
		})
	})

	Context("scale-in", func() {
		It("should delete replicas and update the status.replicas field because spec.replicas value was updated with lower value", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), managedSeedSet)).To(Succeed())
				g.Expect(managedSeedSet.Status.Replicas).To(Equal(int32(1)))
			}).Should(Succeed())

			By("Scale-up the Replicas to 2")
			patch := client.MergeFrom(managedSeedSet.DeepCopy())
			managedSeedSet.Spec.Replicas = ptr.To[int32](2)
			Expect(testClient.Patch(ctx, managedSeedSet, patch)).To(Succeed())

			By("Make one replica ready, to enable controller to pick the next replica")
			makeReplicaReady()

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), managedSeedSet)).To(Succeed())
				g.Expect(managedSeedSet.Status.ReadyReplicas).To(Equal(int32(1)))
				g.Expect(managedSeedSet.Status.PendingReplica).NotTo(BeNil())
				g.Expect(managedSeedSet.Status.Replicas).To(Equal(int32(2)))
			}).Should(Succeed())

			By("Scale-down the Replicas to 1")
			patch = client.MergeFrom(managedSeedSet.DeepCopy())
			managedSeedSet.Spec.Replicas = ptr.To[int32](1)
			Expect(testClient.Patch(ctx, managedSeedSet, patch)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), managedSeedSet)).To(Succeed())
				g.Expect(managedSeedSet.Status.ReadyReplicas).To(Equal(int32(1)))
				g.Expect(managedSeedSet.Status.PendingReplica).To(BeNil())
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
