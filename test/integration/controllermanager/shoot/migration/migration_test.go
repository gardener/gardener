// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package migration_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/migration"
	"github.com/gardener/gardener/pkg/controllerutils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Shoot Migration controller tests", Ordered, func() {
	var (
		shoot           *gardencorev1beta1.Shoot
		sourceSeed      *gardencorev1beta1.Seed
		destinationSeed *gardencorev1beta1.Seed
	)

	BeforeEach(func() {
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: ptr.To("secretbinding"),
				CloudProfileName:  ptr.To("cloudprofile1"),
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
				DNS: &gardencorev1beta1.DNS{
					Domain: ptr.To("some-domain.example.com"),
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.25.1",
				},
				Networking: &gardencorev1beta1.Networking{
					Type: ptr.To("foo-networking"),
				},
				SeedName: ptr.To("source-seed"),
			},
		}

		sourceSeed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{Name: "source-seed"},
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

		destinationSeed = sourceSeed.DeepCopy()
		destinationSeed.Name = "destination-seed"

		By("Create Shoot")
		Expect(testClient.Create(ctx, shoot)).To(Succeed())

		patch := client.MergeFrom(shoot.DeepCopy())
		shoot.Status.SeedName = shoot.Spec.SeedName
		Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

		log.Info("Created Shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

		By("Create source Seed")
		Expect(testClient.Create(ctx, sourceSeed)).To(Succeed())
		log.Info("Created source Seed for test", "seed", client.ObjectKeyFromObject(sourceSeed))
		sourceSeed.Status.Gardener = &gardencorev1beta1.Gardener{Version: "foo"}
		Expect(testClient.Status().Update(ctx, sourceSeed)).To(Succeed())

		By("Create destination Seed")
		Expect(testClient.Create(ctx, destinationSeed)).To(Succeed())
		log.Info("Created destination Seed for test", "seed", client.ObjectKeyFromObject(destinationSeed))

		DeferCleanup(func() {
			By("Delete Shoot")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot))).To(Succeed())

			By("Ensure Shoot is gone")
			Eventually(func() error { return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot) }).Should(BeNotFoundError())

			By("Delete Seeds")
			Expect(client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, testClient, sourceSeed))).To(Succeed())
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, sourceSeed))).To(Succeed())
			Expect(client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, testClient, destinationSeed))).To(Succeed())
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, destinationSeed))).To(Succeed())

			By("Ensure Seeds are gone")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(sourceSeed), sourceSeed)).Should(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(destinationSeed), destinationSeed)).Should(BeNotFoundError())
			}).Should(Succeed())
		})
	})

	When("controller is starting and shoot is being restored", Ordered, func() {
		It("should prepare the shoot before controller is started", func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeRestore}
			shoot.Status.Constraints = []gardencorev1beta1.Condition{{Type: gardencorev1beta1.ShootReadyForMigration}}
			shoot.Status.SeedName = shoot.Spec.SeedName
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())
		})

		It("should successfully add and start the controller", func() {
			Expect((&migration.Reconciler{
				Config: controllermanagerconfigv1alpha1.ShootMigrationControllerConfiguration{
					ConcurrentSyncs: ptr.To(5),
				},
			}).AddToManager(mgr)).To(Succeed())
		})

		It("should remove the constraint when it is still present", func() {
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Status.Constraints
			}).Should(BeEmpty())
		})
	})

	When("the last operation already indicates restore has started", func() {
		It("should do nothing when the constraint is already removed", func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeRestore}
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

			Consistently(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Status.Constraints
			}).Should(BeEmpty())
		})

		It("should remove the constraint when it is still present", func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeRestore}
			shoot.Status.Constraints = []gardencorev1beta1.Condition{{Type: gardencorev1beta1.ShootReadyForMigration}}
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Status.Constraints
			}).Should(BeEmpty())
		})
	})

	When("the destination seed is being deleted", func() {
		It("should set the constraint to False", func() {
			By("Delete destination Seed")
			destinationSeed.Finalizers = []string{"foo"}
			Expect(testClient.Update(ctx, destinationSeed)).To(Succeed())
			Expect(testClient.Delete(ctx, destinationSeed)).To(Succeed())

			By("Add destination Seed reference back to Shoot")
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Spec.SeedName = &destinationSeed.Name
			Expect(testClient.SubResource("binding").Patch(ctx, shoot, patch)).To(Succeed())

			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Status.Constraints
			}).Should(ContainCondition(
				OfType(gardencorev1beta1.ShootReadyForMigration),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("DestinationSeedInDeletion"),
				WithMessageSubstrings("Seed is being deleted"),
			))
		})
	})

	It("should set the constraint to False because destination seed is unready", func() {
		By("Mark destination Seed as unready")
		destinationSeed.Status.Gardener = &gardencorev1beta1.Gardener{Version: "bar"}
		Expect(testClient.Status().Update(ctx, destinationSeed)).To(Succeed())

		By("Trigger migration controller")
		patch := client.MergeFrom(shoot.DeepCopy())
		shoot.Spec.SeedName = &destinationSeed.Name
		Expect(testClient.SubResource("binding").Patch(ctx, shoot, patch)).To(Succeed())

		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			return shoot.Status.Constraints
		}).Should(ContainCondition(
			OfType(gardencorev1beta1.ShootReadyForMigration),
			WithStatus(gardencorev1beta1.ConditionFalse),
			WithReason("DestinationSeedUnready"),
			WithMessageSubstrings("observing Gardener version not up to date (bar/foo)"),
		))
	})

	It("should set the constraint to True because destination seed is ready", func() {
		By("Mark destination Seed as ready")
		destinationSeed.Status.Gardener = sourceSeed.Status.Gardener
		destinationSeed.Status.ObservedGeneration = destinationSeed.Generation
		destinationSeed.Status.Conditions = []gardencorev1beta1.Condition{
			{Type: gardencorev1beta1.SeedGardenletReady, Status: gardencorev1beta1.ConditionTrue},
			{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
		}
		Expect(testClient.Status().Update(ctx, destinationSeed)).To(Succeed())

		By("Trigger migration controller")
		patch := client.MergeFrom(shoot.DeepCopy())
		shoot.Spec.SeedName = &destinationSeed.Name
		Expect(testClient.SubResource("binding").Patch(ctx, shoot, patch)).To(Succeed())

		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			return shoot.Status.Constraints
		}).Should(ContainCondition(
			OfType(gardencorev1beta1.ShootReadyForMigration),
			WithStatus(gardencorev1beta1.ConditionTrue),
			WithReason("DestinationSeedReady"),
			WithMessageSubstrings("Destination seed cluster is ready for the shoot migration"),
		))
	})
})
