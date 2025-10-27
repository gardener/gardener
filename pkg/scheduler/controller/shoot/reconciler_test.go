// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/api/indexer"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
)

var _ = Describe("Scheduler_Control", func() {
	var (
		ctx              = context.Background()
		log              logr.Logger
		fakeGardenClient client.Client

		reconciler   *Reconciler
		cloudProfile *gardencorev1beta1.CloudProfile
		project      *gardencorev1beta1.Project
		seed         *gardencorev1beta1.Seed
		shoot        *gardencorev1beta1.Shoot

		schedulerConfiguration schedulerconfigv1alpha1.SchedulerConfiguration

		providerType     = "foo"
		cloudProfileName = "cloudprofile-1"
		seedName         = "seed-1"
		region           = "europe"

		cloudProfileBase = gardencorev1beta1.CloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name: cloudProfileName,
			},
		}
		projectBase = gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: "project-1",
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: ptr.To("my-namespace"),
			},
		}
		seedBase = gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: seedName,
			},
			Spec: gardencorev1beta1.SeedSpec{
				Provider: gardencorev1beta1.SeedProvider{
					Type:   providerType,
					Region: region,
				},
				Networks: gardencorev1beta1.SeedNetworks{
					Nodes:    ptr.To("10.10.0.0/16"),
					Pods:     "10.20.0.0/16",
					Services: "10.30.0.0/16",
				},
				Settings: &gardencorev1beta1.SeedSettings{
					Scheduling: &gardencorev1beta1.SeedSettingScheduling{
						Visible: true,
					},
				},
			},
			Status: gardencorev1beta1.SeedStatus{
				Conditions: []gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.GardenletReady,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
				LastOperation: &gardencorev1beta1.LastOperation{},
			},
		}
		shootBase = gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot",
				Namespace: "my-namespace",
			},
			Spec: gardencorev1beta1.ShootSpec{
				CloudProfileName: &cloudProfileName,
				Region:           region,
				Provider: gardencorev1beta1.Provider{
					Type: providerType,
					Workers: []gardencorev1beta1.Worker{
						{
							Name: "foo",
						},
					},
				},
				Networking: &gardencorev1beta1.Networking{
					Nodes:    ptr.To("10.40.0.0/16"),
					Pods:     ptr.To("10.50.0.0/16"),
					Services: ptr.To("10.60.0.0/16"),
				},
			},
		}

		schedulerConfigurationBase = schedulerconfigv1alpha1.SchedulerConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: schedulerconfigv1alpha1.SchemeGroupVersion.String(),
				Kind:       "SchedulerConfiguration",
			},
			Schedulers: schedulerconfigv1alpha1.SchedulerControllerConfiguration{
				Shoot: &schedulerconfigv1alpha1.ShootSchedulerConfiguration{
					Strategy: schedulerconfigv1alpha1.SameRegion,
				},
			},
			FeatureGates: map[string]bool{},
		}
	)

	BeforeEach(func() {
		log = logr.Discard()
		fakeGardenClient = fakeclient.
			NewClientBuilder().
			WithScheme(kubernetes.GardenScheme).
			WithIndex(&gardencorev1beta1.Project{}, gardencore.ProjectNamespace, indexer.ProjectNamespaceIndexerFunc).
			Build()
	})

	JustBeforeEach(func() {
		reconciler = &Reconciler{
			Client: fakeGardenClient,
			Config: schedulerConfiguration.Schedulers.Shoot,
		}
	})

	Context("SEED DETERMINATION - Shoot does not reference a Seed - find an adequate one using 'Same Region' seed determination strategy", func() {
		BeforeEach(func() {
			cloudProfile = cloudProfileBase.DeepCopy()
			project = projectBase.DeepCopy()
			seed = seedBase.DeepCopy()
			shoot = shootBase.DeepCopy()
			schedulerConfiguration = *schedulerConfigurationBase.DeepCopy()
			// no seed referenced
			shoot.Spec.SeedName = nil
		})

		// PASS

		It("should find a seed cluster 1) 'Same Region' seed determination strategy 2) referencing the same profile 3) same region 4) indicating availability", func() {
			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(seed.Name))
		})

		It("should find the best seed cluster 1) 'Same Region' seed determination strategy 2) referencing the same profile 3) same region 4) indicating availability", func() {
			secondSeed := seedBase
			secondSeed.Name = "seed-2"

			secondShoot := shootBase
			secondShoot.Name = "shoot-2"
			// first seed references more shoots then seed-2 -> expect seed-2 to be selected
			secondShoot.Spec.SeedName = &seed.Name

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, &secondSeed)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, shoot)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, &secondShoot)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(secondSeed.Name))
		})

		It("should find a multi-zonal seed cluster for a shoot with failure tolerance type 'zone'", func() {
			secondSeed := seedBase
			secondSeed.Name = "seed-multi-zonal"
			secondSeed.Spec.Provider.Zones = []string{"1", "2", "3"}

			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}}

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, &secondSeed)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, shoot)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(secondSeed.Name))
		})

		// FAIL

		It("should fail because it cannot find a seed cluster 1) 'Same Region' seed determination strategy 2) region that no seed supports", func() {
			shoot.Spec.Region = "another-region"

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster (due to no zones) for a shoot with failure tolerance type 'zone'", func() {
			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
				HighAvailability: &gardencorev1beta1.HighAvailability{
					FailureTolerance: gardencorev1beta1.FailureTolerance{
						Type: gardencorev1beta1.FailureToleranceTypeZone,
					},
				},
			}

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, shoot)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).To(MatchError("none of the 1 seeds has at least 3 zones for hosting a shoot control plane with failure tolerance type 'zone'"))
			Expect(bestSeed).To(BeNil())
		})

		It("should fail when the only available seed has < 3 zones for a shoot with failure tolerance type 'zone'", func() {
			seed.Spec.Provider.Zones = []string{"1", "2"}

			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
				HighAvailability: &gardencorev1beta1.HighAvailability{
					FailureTolerance: gardencorev1beta1.FailureTolerance{
						Type: gardencorev1beta1.FailureToleranceTypeZone,
					},
				},
			}

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, shoot)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).To(MatchError("none of the 1 seeds has at least 3 zones for hosting a shoot control plane with failure tolerance type 'zone'"))
			Expect(bestSeed).To(BeNil())
		})

		It("should find a seed because multi-zonal seeds can be used for shoots with failure tolerance type 'node'", func() {
			multiZonalSeed := seedBase
			multiZonalSeed.Name = "seed-multi-zonal"
			multiZonalSeed.Spec.Provider.Zones = []string{"1", "2"}

			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeNode}}}

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, &multiZonalSeed)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, shoot)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(multiZonalSeed.Name))
		})

		It("should find a seed because multi-zonal seeds can be used for non-HA shoots", func() {
			multiZonalSeed := seedBase
			multiZonalSeed.Name = "seed-multi-zonal"
			multiZonalSeed.Spec.Provider.Zones = []string{"1", "2"}

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, &multiZonalSeed)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, shoot)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(multiZonalSeed.Name))
		})
	})

	Context("SEED DETERMINATION - Shoot does not reference a Seed - find an adequate one using 'MinimalDistance' seed determination strategy", func() {
		var anotherType = "another-type"
		var anotherRegion = "another-region"

		BeforeEach(func() {
			seed = seedBase.DeepCopy()
			shoot = shootBase.DeepCopy()
			shoot.Spec.Provider.Type = anotherType
			cloudProfile = cloudProfileBase.DeepCopy()
			cloudProfile.Spec.Type = anotherType
			project = projectBase.DeepCopy()
			schedulerConfiguration = *schedulerConfigurationBase.DeepCopy()
			schedulerConfiguration.Schedulers.Shoot.Strategy = schedulerconfigv1alpha1.MinimalDistance
			// no seed referenced
			shoot.Spec.SeedName = nil
		})

		It("should succeed because it cannot find a seed cluster 1) 'MinimalDistance' seed determination strategy 2) default match", func() {
			seed.Spec.Provider.Type = anotherType
			shoot.Spec.Region = anotherRegion

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed).NotTo(BeNil())
		})

		It("should succeed because it cannot find a seed cluster 1) 'MinimalDistance' seed determination strategy 2) cross provider", func() {
			cloudProfile.Spec.SeedSelector = &gardencorev1beta1.SeedSelector{
				ProviderTypes: []string{providerType},
			}
			shoot.Spec.Region = anotherRegion

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed).NotTo(BeNil())
		})

		It("should succeed because it cannot find a seed cluster 1) 'MinimalDistance' seed determination strategy 2) any provider", func() {
			cloudProfile.Spec.SeedSelector = &gardencorev1beta1.SeedSelector{
				ProviderTypes: []string{"*"},
			}
			shoot.Spec.Region = anotherRegion

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed).NotTo(BeNil())
		})

		It("should succeed because it cannot find a seed cluster 1) 'MinimalDistance' seed determination strategy 2) matching labels", func() {
			cloudProfile.Spec.SeedSelector = &gardencorev1beta1.SeedSelector{
				LabelSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"select": "true",
					},
				},
				ProviderTypes: []string{"*"},
			}
			seed.Labels = map[string]string{"select": "true"}
			shoot.Spec.Region = anotherRegion

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed).NotTo(BeNil())
		})

		// FAIL

		It("should fail because it cannot find a seed cluster 1) 'MinimalDistance' seed determination strategy 2) no matching provider", func() {
			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster 1) 'MinimalDistance' seed determination strategy 2) no matching labels", func() {
			cloudProfile.Spec.SeedSelector = &gardencorev1beta1.SeedSelector{
				LabelSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"select": "true",
					},
				},
				ProviderTypes: []string{providerType},
			}
			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster 1) 'MinimalDistance' seed determination strategy 2) matching labels but not type", func() {
			cloudProfile.Spec.SeedSelector = &gardencorev1beta1.SeedSelector{
				LabelSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"select": "true",
					},
				},
			}
			seed.Labels = map[string]string{"select": "true"}

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})
	})

	Context("SEED DETERMINATION - Shoot does not reference a Seed - find an adequate one using 'Minimal Distance' seed determination strategy", func() {
		BeforeEach(func() {
			cloudProfile = cloudProfileBase.DeepCopy()
			project = projectBase.DeepCopy()
			seed = seedBase.DeepCopy()
			shoot = shootBase.DeepCopy()
			schedulerConfiguration = *schedulerConfigurationBase.DeepCopy()
			// no seed referenced
			shoot.Spec.SeedName = nil
			schedulerConfiguration.Schedulers.Shoot.Strategy = schedulerconfigv1alpha1.MinimalDistance
		})

		It("should find a seed cluster 1) referencing the same profile 2) same region 3) indicating availability", func() {
			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(seedName))
		})

		It("should find a seed cluster from other region: shoot in non-existing region, only one seed existing", func() {
			anotherRegion := "another-region"
			shoot.Spec.Region = anotherRegion

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(seedName))
			// verify that shoot is in another region than the seed
			Expect(shoot.Spec.Region).NotTo(Equal(bestSeed.Spec.Provider.Region))
		})

		It("should find a seed cluster from other region: shoot in non-existing region, multiple seeds existing", func() {
			// add 3 seeds with different names and regions
			seed.Spec.Provider.Region = "europe-north1"

			secondSeed := seedBase
			secondSeed.Name = "seed-2"
			secondSeed.Spec.Provider.Region = "europe-west1"

			thirdSeed := seedBase
			thirdSeed.Name = "seed-3"
			thirdSeed.Spec.Provider.Region = "asia-south1"

			// define shoot to be lexicographically 'closer' to the second seed
			anotherRegion := "europe-west3"
			shoot.Spec.Region = anotherRegion

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, &secondSeed)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, &thirdSeed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(secondSeed.Name))
			// verify that shoot is in another region than the chosen seed
			Expect(shoot.Spec.Region).NotTo(Equal(bestSeed.Spec.Provider.Region))
		})

		It("should pick candidate with least shoots deployed", func() {
			secondSeed := seedBase
			secondSeed.Name = "seed-2"

			secondShoot := shootBase
			secondShoot.Name = "shoot-2"
			// first seed references more shoots then seed-2 -> expect seed-2 to be selected
			secondShoot.Spec.SeedName = &seed.Name

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, &secondSeed)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, shoot)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, &secondShoot)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(secondSeed.Name))
		})

		It("should find seed cluster that matches the seed selector of the CloudProfile and is from another region", func() {
			newCloudProfile := cloudProfile.DeepCopy()
			newCloudProfile.Name = "cloudprofile2"
			newCloudProfile.Spec.SeedSelector = &gardencorev1beta1.SeedSelector{
				LabelSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"environment": "two",
					},
				},
			}
			newCloudProfile.Spec.Regions = []gardencorev1beta1.Region{{Name: "name: eu-nl-1"}}

			// seeds
			oldSeedEnvironment1 := seed.DeepCopy()
			oldSeedEnvironment1.Spec.Provider.Type = "some-type"
			oldSeedEnvironment1.Spec.Provider.Region = "eu-de-200"
			oldSeedEnvironment1.Name = "seed1"
			oldSeedEnvironment1.Labels = map[string]string{"environment": "one"}

			newSeedEnvironment2 := seed.DeepCopy()
			newSeedEnvironment2.Spec.Provider.Type = "some-type"
			newSeedEnvironment2.Spec.Provider.Region = "eu-nl-1"
			newSeedEnvironment2.Name = "seed2"
			newSeedEnvironment2.Labels = map[string]string{"environment": "two"}

			// shoot
			testShoot := shoot
			testShoot.Spec.Region = "eu-de-2"
			testShoot.Spec.CloudProfileName = ptr.To("cloudprofile2")
			testShoot.Spec.Provider.Type = "some-type"

			Expect(fakeGardenClient.Create(ctx, newCloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, oldSeedEnvironment1)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, newSeedEnvironment2)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, testShoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(newSeedEnvironment2.Name))
		})

		It("should find seed cluster that matches the seed selector of the Shoot and is from another region", func() {
			newCloudProfile := cloudProfile
			newCloudProfile.Name = "cloudprofile2"
			newCloudProfile.Spec.SeedSelector = &gardencorev1beta1.SeedSelector{
				LabelSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"environment": "two",
					},
				},
			}
			newCloudProfile.Spec.Regions = []gardencorev1beta1.Region{{Name: "name: eu-nl-1"}}

			// seeds
			oldSeedEnvironment1 := seed.DeepCopy()
			oldSeedEnvironment1.Spec.Provider.Type = "some-type"
			oldSeedEnvironment1.Spec.Provider.Region = "eu-de-200"
			oldSeedEnvironment1.Name = "seed1"
			oldSeedEnvironment1.Labels = map[string]string{"environment": "one"}

			newSeedEnvironment2 := seed.DeepCopy()
			newSeedEnvironment2.Spec.Provider.Type = "some-type"
			newSeedEnvironment2.Spec.Provider.Region = "eu-nl-1"
			newSeedEnvironment2.Name = "seed2"
			newSeedEnvironment2.Labels = map[string]string{"environment": "two"}

			newSeedEnvironment3 := seed.DeepCopy()
			newSeedEnvironment3.Spec.Provider.Type = "some-type"
			newSeedEnvironment3.Spec.Provider.Region = "eu-nl-4"
			newSeedEnvironment3.Name = "seed3"
			newSeedEnvironment3.Labels = map[string]string{"environment": "two", "my-preferred": "seed"}

			// shoot
			testShoot := shoot.DeepCopy()
			testShoot.Spec.Region = "eu-de-2"
			testShoot.Spec.CloudProfileName = ptr.To("cloudprofile2")
			testShoot.Spec.Provider.Type = "some-type"
			testShoot.Spec.SeedSelector = &gardencorev1beta1.SeedSelector{
				LabelSelector: metav1.LabelSelector{MatchLabels: map[string]string{"my-preferred": "seed"}},
			}

			Expect(fakeGardenClient.Create(ctx, newCloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, oldSeedEnvironment1)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, newSeedEnvironment2)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, newSeedEnvironment3)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, testShoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(newSeedEnvironment3.Name))
		})

		It("should find seed cluster with enough available capacity for shoots", func() {
			seed.Status.Allocatable = corev1.ResourceList{
				gardencorev1beta1.ResourceShoots: resource.MustParse("1"),
			}

			secondSeed := seedBase
			secondSeed.Name = "seed-2"
			secondSeed.Status.Allocatable = corev1.ResourceList{
				gardencorev1beta1.ResourceShoots: resource.MustParse("2"),
			}

			secondShoot := shootBase
			secondShoot.Name = "shoot-2"
			secondShoot.Spec.SeedName = &seed.Name

			thirdShoot := shootBase
			thirdShoot.Name = "shoot-3"
			thirdShoot.Spec.SeedName = &secondSeed.Name

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, &secondSeed)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, shoot)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, &secondShoot)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, &thirdShoot)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(secondSeed.Name))
		})
	})

	Context("SEED DETERMINATION - Shoot does not reference a Seed - find an adequate one using default seed determination strategy", func() {
		BeforeEach(func() {
			cloudProfile = cloudProfileBase.DeepCopy()
			project = projectBase.DeepCopy()
			seed = seedBase.DeepCopy()
			shoot = shootBase.DeepCopy()
			schedulerConfiguration = *schedulerConfigurationBase.DeepCopy()
			// no seed referenced
			shoot.Spec.SeedName = nil
			schedulerConfiguration.Schedulers.Shoot.Strategy = schedulerconfigv1alpha1.Default
		})

		It("should find a seed cluster 1) referencing the same profile 2) same region 3) indicating availability", func() {
			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(seedName))
		})

		It("should find a seed cluster 1) referencing the same profile 2) same region 3) indicating availability 4) using shoot default networks", func() {
			seed.Spec.Networks.ShootDefaults = &gardencorev1beta1.ShootNetworks{
				Pods:     ptr.To("10.50.0.0/16"),
				Services: ptr.To("10.60.0.0/16"),
			}
			shoot.Spec.Networking.Pods = nil
			shoot.Spec.Networking.Services = nil

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(seedName))
		})

		It("should find the best seed cluster 1) referencing the same profile 2) same region 3) indicating availability", func() {
			secondSeed := seedBase
			secondSeed.Name = "seed-2"

			secondShoot := shootBase
			secondShoot.Name = "shoot-2"
			secondShoot.Spec.SeedName = &seed.Name

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, &secondSeed)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, shoot)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, &secondShoot)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(secondSeed.Name))
		})

		It("should find the best seed cluster even with network disjointedness (non-HA control plane)", func() {
			shoot.Spec.Networking = &gardencorev1beta1.Networking{
				Pods:     &seed.Spec.Networks.Pods,
				Services: &seed.Spec.Networks.Services,
				Nodes:    seed.Spec.Networks.Nodes,
			}

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).To(Not(HaveOccurred()))
			Expect(bestSeed.Name).To(Equal(seed.Name))
		})

		It("should find the best seed cluster that matches the domain of the shoot", func() {
			seedWithMatchingDomain := seedBase.DeepCopy()
			seedWithMatchingDomain.Name = "seed-with-matching-domain"
			seedWithMatchingDomain.Spec.DNS = gardencorev1beta1.SeedDNS{
				Defaults: []gardencorev1beta1.SeedDNSProviderConfig{
					{Domain: "example.com"},
				},
			}

			seedWithDifferentDomain := seedBase.DeepCopy()
			seedWithDifferentDomain.Name = "seed-with-different-domain"
			seedWithDifferentDomain.Spec.DNS = gardencorev1beta1.SeedDNS{
				Defaults: []gardencorev1beta1.SeedDNSProviderConfig{
					{Domain: "other.com"},
				},
			}

			shoot.Spec.DNS = &gardencorev1beta1.DNS{
				Domain: ptr.To(fmt.Sprintf("%s.%s.example.com", shoot.Name, project.Name)),
			}

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seedWithMatchingDomain)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seedWithDifferentDomain)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(seedWithMatchingDomain.Name))
		})

		// FAIL

		It("should fail because it cannot find a seed cluster due to network disjointedness (HA control plane)", func() {
			shoot.Spec.Networking = &gardencorev1beta1.Networking{
				Pods:     &seed.Spec.Networks.Pods,
				Services: &seed.Spec.Networks.Services,
				Nodes:    seed.Spec.Networks.Nodes,
			}

			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
				HighAvailability: &gardencorev1beta1.HighAvailability{
					FailureTolerance: gardencorev1beta1.FailureTolerance{
						Type: gardencorev1beta1.FailureToleranceTypeZone,
					},
				},
			}

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster due to non-tolerated taints", func() {
			seed2 := seed.DeepCopy()
			seed2.Name = "seed-2"
			seed.Spec.Taints = []gardencorev1beta1.SeedTaint{{Key: "foo"}}
			seed2.Spec.Taints = []gardencorev1beta1.SeedTaint{{Key: "bar"}}
			shoot.Spec.Tolerations = nil

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed2)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).To(MatchError("0/2 seed cluster candidate(s) are eligible for scheduling: {seed-1 => shoot does not tolerate the seed's taints, seed-2 => shoot does not tolerate the seed's taints}"))
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster due to no available capacity for shoots", func() {
			seed.Status.Allocatable = corev1.ResourceList{
				gardencorev1beta1.ResourceShoots: resource.MustParse("1"),
			}
			secondShoot := shootBase
			secondShoot.Name = "shoot-2"
			secondShoot.Spec.SeedName = &seed.Name

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, shoot)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, &secondShoot)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster due to region that no seed supports", func() {
			shoot.Spec.Region = "another-region"

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because the cloudprofile used by the shoot doesn't select any seed candidate", func() {
			cloudProfile.Spec.SeedSelector = &gardencorev1beta1.SeedSelector{
				LabelSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"foo": "bar",
					},
				},
			}
			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because the shoot doesn't select any seed candidate", func() {
			shoot.Spec.SeedSelector = &gardencorev1beta1.SeedSelector{
				LabelSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"foo": "bar",
					},
				},
			}

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster due to invalid profile", func() {
			shoot.Spec.CloudProfileName = ptr.To("another-profile")

			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster due to missing project", func() {
			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster due to gardenlet not ready", func() {
			seed.Status.Conditions = []gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.GardenletReady,
					Status: gardencorev1beta1.ConditionFalse,
				},
			}

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster reconciled at least once before", func() {
			seed.Status.LastOperation = nil

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster due to invisibility", func() {
			seed.Spec.Settings = &gardencorev1beta1.SeedSettings{
				Scheduling: &gardencorev1beta1.SeedSettingScheduling{
					Visible: false,
				},
			}

			Expect(fakeGardenClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, project)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, seed)).To(Succeed())

			bestSeed, err := reconciler.DetermineSeed(ctx, log, shoot)
			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})
	})

	Context("#DetermineBestSeedCandidate", func() {
		BeforeEach(func() {
			seed = seedBase.DeepCopy()
			shoot = shootBase.DeepCopy()
			schedulerConfiguration = *schedulerConfigurationBase.DeepCopy()
			// no seed referenced
			shoot.Spec.SeedName = nil
			schedulerConfiguration.Schedulers.Shoot.Strategy = schedulerconfigv1alpha1.MinimalDistance
		})

		It("should find two seeds candidates having the same amount of matching characters", func() {
			oldSeedEnvironment1 := *seed
			oldSeedEnvironment1.Spec.Provider.Type = "some-type"
			oldSeedEnvironment1.Spec.Provider.Region = "eu-de-200"
			oldSeedEnvironment1.Name = "seed1"

			newSeedEnvironment2 := *seed
			newSeedEnvironment2.Spec.Provider.Type = "some-type"
			newSeedEnvironment2.Spec.Provider.Region = "eu-de-2111"
			newSeedEnvironment2.Name = "seed2"

			otherSeedEnvironment2 := *seed
			otherSeedEnvironment2.Spec.Provider.Type = "some-type"
			otherSeedEnvironment2.Spec.Provider.Region = "eu-nl-1"
			otherSeedEnvironment2.Name = "xyz"

			// shoot
			testShoot := shoot
			testShoot.Spec.Region = "eu-de-2xzxzzx"
			testShoot.Spec.CloudProfileName = ptr.To("cloudprofile2")
			testShoot.Spec.Provider.Type = "some-type"

			candidates, err := applyStrategy(log, testShoot, []gardencorev1beta1.Seed{newSeedEnvironment2, oldSeedEnvironment1, otherSeedEnvironment2}, schedulerConfiguration.Schedulers.Shoot.Strategy, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(candidates).To(HaveLen(2))
			Expect(candidates[0].Name).To(Equal(newSeedEnvironment2.Name))
			Expect(candidates[1].Name).To(Equal(oldSeedEnvironment1.Name))
		})

		It("should find single seed candidate", func() {
			oldSeedEnvironment1 := *seed
			oldSeedEnvironment1.Spec.Provider.Type = "some-type"
			oldSeedEnvironment1.Spec.Provider.Region = "eu-de-200"
			oldSeedEnvironment1.Name = "seed1"

			newSeedEnvironment2 := *seed
			newSeedEnvironment2.Spec.Provider.Type = "some-type"
			newSeedEnvironment2.Spec.Provider.Region = "eu-de-2111"
			newSeedEnvironment2.Name = "seed2"

			otherSeedEnvironment2 := *seed
			otherSeedEnvironment2.Spec.Provider.Type = "some-type"
			otherSeedEnvironment2.Spec.Provider.Region = "eu-nl-1"
			otherSeedEnvironment2.Name = "xyz"

			// shoot
			testShoot := shoot
			testShoot.Spec.Region = "eu-de-20"
			testShoot.Spec.CloudProfileName = ptr.To("cloudprofile2")
			testShoot.Spec.Provider.Type = "some-type"

			candidates, err := applyStrategy(log, testShoot, []gardencorev1beta1.Seed{newSeedEnvironment2, oldSeedEnvironment1, otherSeedEnvironment2}, schedulerConfiguration.Schedulers.Shoot.Strategy, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(candidates).To(HaveLen(1))
			Expect(candidates[0].Name).To(Equal(oldSeedEnvironment1.Name))
		})
	})

	Context("filterSeedsMatchingDomain", func() {
		var (
			projectName                    = "test-project"
			seedWithDefaultDomain          *gardencorev1beta1.Seed
			seedWithoutDefaultDomain       *gardencorev1beta1.Seed
			seedWithMultipleDefaultDomains *gardencorev1beta1.Seed
		)

		BeforeEach(func() {
			seedWithDefaultDomain = seedBase.DeepCopy()
			seedWithDefaultDomain.Name = "seed-with-default-domain"
			seedWithDefaultDomain.Spec.DNS = gardencorev1beta1.SeedDNS{
				Defaults: []gardencorev1beta1.SeedDNSProviderConfig{
					{Domain: "example.com"},
				},
			}

			seedWithoutDefaultDomain = seedBase.DeepCopy()
			seedWithoutDefaultDomain.Name = "seed-without-default-domain"
			seedWithoutDefaultDomain.Spec.DNS = gardencorev1beta1.SeedDNS{}

			seedWithMultipleDefaultDomains = seedBase.DeepCopy()
			seedWithMultipleDefaultDomains.Name = "seed-with-multiple-default-domains"
			seedWithMultipleDefaultDomains.Spec.DNS = gardencorev1beta1.SeedDNS{
				Defaults: []gardencorev1beta1.SeedDNSProviderConfig{
					{Domain: "example.com"},
					{Domain: "test.org"},
				},
			}
		})

		It("should return all seeds when shoot has no DNS configuration", func() {
			testShoot := shootBase.DeepCopy()
			testShoot.Spec.DNS = nil

			seedList := []gardencorev1beta1.Seed{*seedWithDefaultDomain, *seedWithoutDefaultDomain}

			result, err := filterSeedsMatchingDomain(seedList, testShoot, projectName)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(2))
		})

		It("should return all seeds when shoot has DNS configuration but no domain", func() {
			testShoot := shootBase.DeepCopy()
			testShoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: nil}

			seedList := []gardencorev1beta1.Seed{*seedWithDefaultDomain, *seedWithoutDefaultDomain}

			result, err := filterSeedsMatchingDomain(seedList, testShoot, projectName)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(2))
		})

		It("should return all seeds when shoot uses a custom domain (not matching any seed default)", func() {
			testShoot := shootBase.DeepCopy()
			testShoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: ptr.To("custom.domain.com")}

			seedList := []gardencorev1beta1.Seed{*seedWithDefaultDomain, *seedWithoutDefaultDomain}

			result, err := filterSeedsMatchingDomain(seedList, testShoot, projectName)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(2))
		})

		It("should return only seeds with matching default domain when shoot uses Gardener-managed domain", func() {
			testShoot := shootBase.DeepCopy()
			testShoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: ptr.To(fmt.Sprintf("%s.%s.example.com", testShoot.Name, projectName))}

			seedList := []gardencorev1beta1.Seed{*seedWithDefaultDomain, *seedWithoutDefaultDomain}

			result, err := filterSeedsMatchingDomain(seedList, testShoot, projectName)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].Name).To(Equal(seedWithDefaultDomain.Name))
		})

		It("should return seeds with multiple default domains when one matches", func() {
			testShoot := shootBase.DeepCopy()
			testShoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: ptr.To(fmt.Sprintf("%s.%s.test.org", testShoot.Name, projectName))}

			seedList := []gardencorev1beta1.Seed{*seedWithDefaultDomain, *seedWithoutDefaultDomain, *seedWithMultipleDefaultDomains}

			result, err := filterSeedsMatchingDomain(seedList, testShoot, projectName)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].Name).To(Equal(seedWithMultipleDefaultDomains.Name))
		})

		It("should fail when shoot uses Gardener-managed domain but wrong format", func() {
			testShoot := shootBase.DeepCopy()
			testShoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: ptr.To("wrong-format.example.com")}

			seedList := []gardencorev1beta1.Seed{*seedWithDefaultDomain, *seedWithoutDefaultDomain}

			result, err := filterSeedsMatchingDomain(seedList, testShoot, projectName)
			Expect(err).To(MatchError(`none of the 2 seeds support the domain "wrong-format.example.com" configured in the shoot specification`))
			Expect(result).To(BeNil())
		})

		It("should fail when shoot uses Gardener-managed domain with correct suffix but wrong shoot name", func() {
			testShoot := shootBase.DeepCopy()
			testShoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: ptr.To(fmt.Sprintf("wrong-name.%s.example.com", projectName))}

			seedList := []gardencorev1beta1.Seed{*seedWithDefaultDomain, *seedWithoutDefaultDomain}

			result, err := filterSeedsMatchingDomain(seedList, testShoot, projectName)
			Expect(err).To(MatchError(fmt.Sprintf(`none of the 2 seeds support the domain "wrong-name.%s.example.com" configured in the shoot specification`, projectName)))
			Expect(result).To(BeNil())
		})

		It("should fail when shoot uses Gardener-managed domain with correct suffix but wrong project name", func() {
			testShoot := shootBase.DeepCopy()
			testShoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: ptr.To(fmt.Sprintf("%s.wrong-project.example.com", testShoot.Name))}

			seedList := []gardencorev1beta1.Seed{*seedWithDefaultDomain, *seedWithoutDefaultDomain}

			result, err := filterSeedsMatchingDomain(seedList, testShoot, projectName)
			Expect(err).To(MatchError(fmt.Sprintf(`none of the 2 seeds support the domain "%s.wrong-project.example.com" configured in the shoot specification`, testShoot.Name)))
			Expect(result).To(BeNil())
		})

		It("should return multiple seeds when multiple seeds support the same domain", func() {
			testShoot := shootBase.DeepCopy()
			testShoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: ptr.To(fmt.Sprintf("%s.%s.example.com", testShoot.Name, projectName))}

			seedWithSameDomain := seedBase.DeepCopy()
			seedWithSameDomain.Name = "seed-with-same-domain"
			seedWithSameDomain.Spec.DNS = gardencorev1beta1.SeedDNS{
				Defaults: []gardencorev1beta1.SeedDNSProviderConfig{
					{Domain: "example.com"},
				},
			}

			seedList := []gardencorev1beta1.Seed{*seedWithDefaultDomain, *seedWithSameDomain, *seedWithoutDefaultDomain}

			result, err := filterSeedsMatchingDomain(seedList, testShoot, projectName)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(2))
			Expect(result[0].Name).To(Equal(seedWithDefaultDomain.Name))
			Expect(result[1].Name).To(Equal(seedWithSameDomain.Name))
		})
	})
})

var _ = DescribeTable("condition is false",
	func(conditionType gardencorev1beta1.ConditionType, deleteCondition, backup bool, expected gomegatypes.GomegaMatcher) {
		var seedBackup *gardencorev1beta1.Backup
		if backup {
			seedBackup = &gardencorev1beta1.Backup{}
		}

		seed := &gardencorev1beta1.Seed{
			Spec: gardencorev1beta1.SeedSpec{
				Backup: seedBackup,
			},
			Status: gardencorev1beta1.SeedStatus{
				Conditions: []gardencorev1beta1.Condition{
					{Type: gardencorev1beta1.GardenletReady, Status: gardencorev1beta1.ConditionTrue},
					{Type: gardencorev1beta1.SeedBackupBucketsReady, Status: gardencorev1beta1.ConditionTrue},
					{Type: gardencorev1beta1.SeedExtensionsReady, Status: gardencorev1beta1.ConditionTrue},
				},
				LastOperation: &gardencorev1beta1.LastOperation{},
			},
		}

		for i, cond := range seed.Status.Conditions {
			if cond.Type == conditionType {
				if deleteCondition {
					seed.Status.Conditions = append(seed.Status.Conditions[:i], seed.Status.Conditions[i+1:]...)
				} else {
					seed.Status.Conditions[i].Status = gardencorev1beta1.ConditionFalse
				}
				break
			}
		}

		Expect(verifySeedReadiness(seed)).To(expected)
	},

	Entry("GardenletReady is missing", gardencorev1beta1.GardenletReady, true, true, BeFalse()),
	Entry("GardenletReady is false", gardencorev1beta1.GardenletReady, false, true, BeFalse()),
	Entry("SeedBackupBucketsReady is missing", gardencorev1beta1.SeedBackupBucketsReady, true, true, BeFalse()),
	Entry("SeedBackupBucketsReady is missing but no backup specified", gardencorev1beta1.SeedBackupBucketsReady, true, false, BeTrue()),
	Entry("SeedBackupBucketsReady is false", gardencorev1beta1.SeedBackupBucketsReady, false, true, BeFalse()),
	Entry("SeedBackupBucketsReady is false but no backup specified", gardencorev1beta1.SeedBackupBucketsReady, false, false, BeTrue()),
	Entry("SeedExtensionsReady is missing", gardencorev1beta1.SeedExtensionsReady, true, true, BeTrue()),
	Entry("SeedExtensionsReady is false", gardencorev1beta1.SeedExtensionsReady, false, true, BeTrue()),
)
