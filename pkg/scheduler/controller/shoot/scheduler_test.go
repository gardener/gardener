// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/scheduler/apis/config"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Scheduler_Control", func() {
	var (
		ctrl *gomock.Controller

		gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory

		cloudProfile gardencorev1beta1.CloudProfile
		seed         gardencorev1beta1.Seed
		shoot        gardencorev1beta1.Shoot

		schedulerConfiguration config.SchedulerConfiguration

		providerType     = "foo"
		cloudProfileName = "cloudprofile-1"
		seedName         = "seed-1"
		region           = "europe"

		cloudProfileBase = gardencorev1beta1.CloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name: cloudProfileName,
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
					Nodes:    makeStrPtr("10.10.0.0/16"),
					Pods:     "10.20.0.0/16",
					Services: "10.30.0.0/16",
				},
			},
			Status: gardencorev1beta1.SeedStatus{
				Conditions: []gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.SeedGardenletReady,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.SeedBootstrapped,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
			},
		}
		shootBase = gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot",
				Namespace: "my-namespace",
			},
			Spec: gardencorev1beta1.ShootSpec{
				CloudProfileName: cloudProfileName,
				Region:           region,
				Provider: gardencorev1beta1.Provider{
					Type: providerType,
				},
				Networking: gardencorev1beta1.Networking{
					Nodes:    makeStrPtr("10.40.0.0/16"),
					Pods:     makeStrPtr("10.50.0.0/16"),
					Services: makeStrPtr("10.60.0.0/16"),
				},
			},
		}

		schedulerConfigurationBase = config.SchedulerConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: config.SchemeGroupVersion.String(),
				Kind:       "SchedulerConfiguration",
			},
			Schedulers: config.SchedulerControllerConfiguration{
				Shoot: &config.ShootSchedulerConfiguration{
					Strategy: config.SameRegion,
				},
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("SEED DETERMINATION - Shoot does not reference a Seed - find an adequate one using 'Same Region' seed determination strategy", func() {
		BeforeEach(func() {
			cloudProfile = *cloudProfileBase.DeepCopy()
			seed = *seedBase.DeepCopy()
			shoot = *shootBase.DeepCopy()
			schedulerConfiguration = *schedulerConfigurationBase.DeepCopy()
			gardenCoreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			// no seed referenced
			shoot.Spec.SeedName = nil
		})

		// PASS

		It("should find a seed cluster 1) 'Same Region' seed determination strategy 2) referencing the same profile 3) same region 4) indicating availability", func() {
			gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			bestSeed, err := determineSeed(&shoot, gardenCoreInformerFactory.Core().V1beta1().Seeds().Lister(), gardenCoreInformerFactory.Core().V1beta1().Shoots().Lister(), gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(seed.Name))
		})

		It("should find the best seed cluster 1) 'Same Region' seed determination strategy 2) referencing the same profile 3) same region 4) indicating availability", func() {
			gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)

			secondSeed := seedBase
			secondSeed.Name = "seed-2"

			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)
			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&secondSeed)

			secondShoot := shootBase
			secondShoot.Name = "shoot-2"
			// first seed references more shoots then seed-2 -> expect seed-2 to be selected
			secondShoot.Spec.SeedName = &seed.Name

			gardenCoreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(&secondShoot)

			bestSeed, err := determineSeed(&shoot, gardenCoreInformerFactory.Core().V1beta1().Seeds().Lister(), gardenCoreInformerFactory.Core().V1beta1().Shoots().Lister(), gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(secondSeed.Name))
		})

		// FAIL

		It("should fail because it cannot find a seed cluster 1) 'Same Region' seed determination strategy 2) region that no seed supports", func() {
			gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)

			shoot.Spec.Region = "another-region"

			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			bestSeed, err := determineSeed(&shoot, gardenCoreInformerFactory.Core().V1beta1().Seeds().Lister(), gardenCoreInformerFactory.Core().V1beta1().Shoots().Lister(), gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})
	})

	Context("SEED DETERMINATION - Shoot does not reference a Seed - find an adequate one using 'Minimal Distance' seed determination strategy", func() {
		BeforeEach(func() {
			cloudProfile = *cloudProfileBase.DeepCopy()
			seed = *seedBase.DeepCopy()
			shoot = *shootBase.DeepCopy()
			schedulerConfiguration = *schedulerConfigurationBase.DeepCopy()
			gardenCoreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			// no seed referenced
			shoot.Spec.SeedName = nil
			schedulerConfiguration.Schedulers.Shoot.Strategy = config.MinimalDistance
		})

		It("should find a seed cluster 1) referencing the same profile 2) same region 3) indicating availability", func() {
			gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			bestSeed, err := determineSeed(&shoot, gardenCoreInformerFactory.Core().V1beta1().Seeds().Lister(), gardenCoreInformerFactory.Core().V1beta1().Shoots().Lister(), gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(seedName))
		})

		It("should find a seed cluster  ) referencing the same profile 2) different region 3) indicating availability 4) only one seed existing", func() {
			gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)

			anotherRegion := "another-region"
			shoot.Spec.Region = anotherRegion

			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			bestSeed, err := determineSeed(&shoot, gardenCoreInformerFactory.Core().V1beta1().Seeds().Lister(), gardenCoreInformerFactory.Core().V1beta1().Shoots().Lister(), gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(seedName))
			// verify that shoot is in another region than the seed
			Expect(shoot.Spec.Region).NotTo(Equal(bestSeed.Spec.Provider.Region))
		})

		It("should find the seed cluster with the minimal distance 1) referencing the same profile 2) different region 3) indicating availability 4) multiple seeds existing", func() {
			gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)

			// add 3 seeds with different names and regions
			seed.Spec.Provider.Region = "europe-north1"

			secondSeed := seedBase
			secondSeed.Name = "seed-2"
			secondSeed.Spec.Provider.Region = "europe-west1"

			thirdSeed := seedBase
			thirdSeed.Name = "seed-3"
			thirdSeed.Spec.Provider.Region = "asia-south1"

			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)
			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&secondSeed)
			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&thirdSeed)

			// define shoot to be lexicographically 'closer' to the second seed
			anotherRegion := "europe-west3"
			shoot.Spec.Region = anotherRegion

			bestSeed, err := determineSeed(&shoot, gardenCoreInformerFactory.Core().V1beta1().Seeds().Lister(), gardenCoreInformerFactory.Core().V1beta1().Shoots().Lister(), gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(secondSeed.Name))
			// verify that shoot is in another region than the chosen seed
			Expect(shoot.Spec.Region).NotTo(Equal(bestSeed.Spec.Provider.Region))
		})

		It("should find the best seed cluster 1) referencing the same profile 2) same region 3) indicating availability 4) multiple seeds existing", func() {
			gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)

			secondSeed := seedBase
			secondSeed.Name = "seed-2"

			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)
			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&secondSeed)

			secondShoot := shootBase
			secondShoot.Name = "shoot-2"
			// first seed references more shoots then seed-2 -> expect seed-2 to be selected
			secondShoot.Spec.SeedName = &seed.Name

			gardenCoreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(&secondShoot)

			bestSeed, err := determineSeed(&shoot, gardenCoreInformerFactory.Core().V1beta1().Seeds().Lister(), gardenCoreInformerFactory.Core().V1beta1().Shoots().Lister(), gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(secondSeed.Name))
		})
	})

	Context("SEED DETERMINATION - Shoot does not reference a Seed - find an adequate one using default seed determination strategy", func() {
		BeforeEach(func() {
			cloudProfile = *cloudProfileBase.DeepCopy()
			seed = *seedBase.DeepCopy()
			shoot = *shootBase.DeepCopy()
			schedulerConfiguration = *schedulerConfigurationBase.DeepCopy()
			gardenCoreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			// no seed referenced
			shoot.Spec.SeedName = nil
			schedulerConfiguration.Schedulers.Shoot.Strategy = config.Default
		})

		It("should find a seed cluster 1) referencing the same profile 2) same region 3) indicating availability", func() {
			gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			bestSeed, err := determineSeed(&shoot, gardenCoreInformerFactory.Core().V1beta1().Seeds().Lister(), gardenCoreInformerFactory.Core().V1beta1().Shoots().Lister(), gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(seedName))
		})

		It("should find a seed cluster 1) referencing the same profile 2) same region 3) indicating availability 4) using shoot default networks", func() {
			gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			shoot.Spec.Networking.Pods = nil
			shoot.Spec.Networking.Services = nil

			seed.Spec.Networks.ShootDefaults = &gardencorev1beta1.ShootNetworks{
				Pods:     makeStrPtr("10.50.0.0/16"),
				Services: makeStrPtr("10.60.0.0/16"),
			}

			bestSeed, err := determineSeed(&shoot, gardenCoreInformerFactory.Core().V1beta1().Seeds().Lister(), gardenCoreInformerFactory.Core().V1beta1().Shoots().Lister(), gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(seedName))
		})

		It("should find the best seed cluster 1) referencing the same profile 2) same region 3) indicating availability", func() {
			gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)

			secondSeed := seedBase
			secondSeed.Name = "seed-2"

			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)
			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&secondSeed)

			secondShoot := shootBase
			secondShoot.Name = "shoot-2"
			secondShoot.Spec.SeedName = &seed.Name

			gardenCoreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(&secondShoot)

			bestSeed, err := determineSeed(&shoot, gardenCoreInformerFactory.Core().V1beta1().Seeds().Lister(), gardenCoreInformerFactory.Core().V1beta1().Shoots().Lister(), gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(secondSeed.Name))
		})

		// FAIL

		It("should fail because it cannot find a seed cluster due to network disjointedness", func() {
			gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			shoot.Spec.Networking = gardencorev1beta1.Networking{
				Pods:     &seed.Spec.Networks.Pods,
				Services: &seed.Spec.Networks.Services,
				Nodes:    seed.Spec.Networks.Nodes,
			}

			bestSeed, err := determineSeed(&shoot, gardenCoreInformerFactory.Core().V1beta1().Seeds().Lister(), gardenCoreInformerFactory.Core().V1beta1().Shoots().Lister(), gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster due to region that no seed supports", func() {
			gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			shoot.Spec.Region = "another-region"

			bestSeed, err := determineSeed(&shoot, gardenCoreInformerFactory.Core().V1beta1().Seeds().Lister(), gardenCoreInformerFactory.Core().V1beta1().Shoots().Lister(), gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because the cloudprofile used by the shoot doesn't select any seed candidate", func() {
			cloudProfile.Spec.SeedSelector = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			}

			gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			bestSeed, err := determineSeed(&shoot, gardenCoreInformerFactory.Core().V1beta1().Seeds().Lister(), gardenCoreInformerFactory.Core().V1beta1().Shoots().Lister(), gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster due to invalid profile", func() {
			gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			shoot.Spec.CloudProfileName = "another-profile"

			bestSeed, err := determineSeed(&shoot, gardenCoreInformerFactory.Core().V1beta1().Seeds().Lister(), gardenCoreInformerFactory.Core().V1beta1().Shoots().Lister(), gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster due to gardenlet not ready", func() {
			gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)

			seed.Status.Conditions = []gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.SeedGardenletReady,
					Status: gardencorev1beta1.ConditionFalse,
				},
				{
					Type:   gardencorev1beta1.SeedBootstrapped,
					Status: gardencorev1beta1.ConditionTrue,
				},
			}
			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			bestSeed, err := determineSeed(&shoot, gardenCoreInformerFactory.Core().V1beta1().Seeds().Lister(), gardenCoreInformerFactory.Core().V1beta1().Shoots().Lister(), gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster due to not bootstrapped", func() {
			gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)

			seed.Status.Conditions = []gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.SeedGardenletReady,
					Status: gardencorev1beta1.ConditionTrue,
				},
				{
					Type:   gardencorev1beta1.SeedBootstrapped,
					Status: gardencorev1beta1.ConditionFalse,
				},
			}
			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			bestSeed, err := determineSeed(&shoot, gardenCoreInformerFactory.Core().V1beta1().Seeds().Lister(), gardenCoreInformerFactory.Core().V1beta1().Shoots().Lister(), gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster due to invisibility", func() {
			gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)

			seed.Spec.Taints = []gardencorev1beta1.SeedTaint{
				{Key: gardencorev1beta1.SeedTaintInvisible},
			}
			gardenCoreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			bestSeed, err := determineSeed(&shoot, gardenCoreInformerFactory.Core().V1beta1().Seeds().Lister(), gardenCoreInformerFactory.Core().V1beta1().Shoots().Lister(), gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})
	})

	Context("Scheduling", func() {
		var (
			shoot = shootBase.DeepCopy()
			seed  = *seedBase.DeepCopy()
		)

		It("should request the scheduling of the shoot to the seed", func() {
			var runtimeClient = mockclient.NewMockClient(ctrl)

			shoot.Spec.SeedName = &seed.Name
			runtimeClient.EXPECT().Update(context.TODO(), shoot).DoAndReturn(func(ctx context.Context, list runtime.Object) error {
				return nil
			})

			executeSchedulingRequest := func(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
				return runtimeClient.Update(ctx, shoot)
			}

			err := UpdateShootToBeScheduledOntoSeed(context.TODO(), shoot, &seed, executeSchedulingRequest)

			Expect(err).NotTo(HaveOccurred())
		})
	})
})

func makeStrPtr(v string) *string {
	c := string(v)
	return &c
}
