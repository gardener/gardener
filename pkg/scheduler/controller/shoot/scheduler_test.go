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

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/scheduler/apis/config"
)

var _ = Describe("Scheduler_Control", func() {
	var (
		ctrl *gomock.Controller

		gardenInformerFactory  gardeninformers.SharedInformerFactory
		seed                   gardenv1beta1.Seed
		shoot                  gardenv1beta1.Shoot
		schedulerConfiguration config.SchedulerConfiguration

		cloudProfileName = "cloudprofile-1"
		seedName         = "seed-1"
		region           = "europe"

		falseVar = false
		trueVar  = true

		seedBase = gardenv1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: seedName,
			},
			Spec: gardenv1beta1.SeedSpec{
				Cloud: gardenv1beta1.SeedCloud{
					Profile: cloudProfileName,
					Region:  region,
				},
				Visible:   &trueVar,
				Protected: &falseVar,
				Networks: gardenv1beta1.SeedNetworks{
					Nodes:    gardencorev1alpha1.CIDR("10.10.0.0/16"),
					Pods:     gardencorev1alpha1.CIDR("10.20.0.0/16"),
					Services: gardencorev1alpha1.CIDR("10.30.0.0/16"),
				},
			},
			Status: gardenv1beta1.SeedStatus{
				Conditions: []gardencorev1alpha1.Condition{
					{
						Type:   gardenv1beta1.SeedAvailable,
						Status: gardencorev1alpha1.ConditionTrue,
					},
				},
			},
		}
		shootBase = gardenv1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot",
				Namespace: "my-namespace",
			},
			Spec: gardenv1beta1.ShootSpec{
				Cloud: gardenv1beta1.Cloud{
					Profile: cloudProfileName,
					Region:  region,
					AWS: &gardenv1beta1.AWSCloud{
						Networks: gardenv1beta1.AWSNetworks{
							K8SNetworks: gardencorev1alpha1.K8SNetworks{
								Nodes:    makeCIDRPtr("10.40.0.0/16"),
								Pods:     makeCIDRPtr("10.50.0.0/16"),
								Services: makeCIDRPtr("10.60.0.0/16"),
							},
						},
					},
				},
			},
		}

		schedulerConfigurationBase = config.SchedulerConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "scheduler.config.gardener.cloud/v1alpha1",
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
			seed = *seedBase.DeepCopy()
			shoot = *shootBase.DeepCopy()
			schedulerConfiguration = *schedulerConfigurationBase.DeepCopy()
			gardenInformerFactory = gardeninformers.NewSharedInformerFactory(nil, 0)
			// no seed referenced
			shoot.Spec.Cloud.Seed = nil
		})

		// PASS

		It("should find a seed cluster 1) 'Same Region' seed determination strategy 2) referencing the same profile 3) same  region 4) indicating availability", func() {
			gardenInformerFactory.Garden().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			bestSeed, err := determineSeed(&shoot, gardenInformerFactory.Garden().V1beta1().Seeds().Lister(), gardenInformerFactory.Garden().V1beta1().Shoots().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(seed.Name))
		})

		It("should find the best seed cluster 1) 'Same Region' seed determination strategy 2) referencing the same profile 3) same  region 4) indicating availability", func() {
			secondSeed := seedBase
			secondSeed.Name = "seed-2"

			gardenInformerFactory.Garden().V1beta1().Seeds().Informer().GetStore().Add(&seed)
			gardenInformerFactory.Garden().V1beta1().Seeds().Informer().GetStore().Add(&secondSeed)

			secondShoot := shootBase
			secondShoot.Name = "shoot-2"
			// first seed references more shoots then seed-2 -> expect seed-2 to be selected
			secondShoot.Spec.Cloud.Seed = &seed.Name

			gardenInformerFactory.Garden().V1beta1().Shoots().Informer().GetStore().Add(&secondShoot)

			bestSeed, err := determineSeed(&shoot, gardenInformerFactory.Garden().V1beta1().Seeds().Lister(), gardenInformerFactory.Garden().V1beta1().Shoots().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(secondSeed.Name))
		})

		// FAIL

		It("should fail because it cannot find a seed cluster  1) 'Same Region' seed determination strategy 2) region that no seed supports", func() {
			shoot.Spec.Cloud.Region = "another-region"

			gardenInformerFactory.Garden().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			bestSeed, err := determineSeed(&shoot, gardenInformerFactory.Garden().V1beta1().Seeds().Lister(), gardenInformerFactory.Garden().V1beta1().Shoots().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})
	})

	Context("SEED DETERMINATION - Shoot does not reference a Seed - find an adequate one using 'Minimal Distance' seed determination strategy", func() {
		BeforeEach(func() {
			seed = *seedBase.DeepCopy()
			shoot = *shootBase.DeepCopy()
			schedulerConfiguration = *schedulerConfigurationBase.DeepCopy()
			gardenInformerFactory = gardeninformers.NewSharedInformerFactory(nil, 0)
			// no seed referenced
			shoot.Spec.Cloud.Seed = nil
			schedulerConfiguration.Schedulers.Shoot.Strategy = config.MinimalDistance
		})

		It("should find a seed cluster 1) referencing the same profile 2) same  region 3) indicating availability", func() {
			gardenInformerFactory.Garden().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			bestSeed, err := determineSeed(&shoot, gardenInformerFactory.Garden().V1beta1().Seeds().Lister(), gardenInformerFactory.Garden().V1beta1().Shoots().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(seedName))
		})

		It("should find a seed cluster  1) referencing the same profile 2) different region 3) indicating availability 4) only one seed existing", func() {
			anotherRegion := "another-region"
			shoot.Spec.Cloud.Region = anotherRegion

			gardenInformerFactory.Garden().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			bestSeed, err := determineSeed(&shoot, gardenInformerFactory.Garden().V1beta1().Seeds().Lister(), gardenInformerFactory.Garden().V1beta1().Shoots().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(seedName))
			// verify that shoot is in another region than the seed
			Expect(shoot.Spec.Cloud.Region).NotTo(Equal(bestSeed.Spec.Cloud.Region))
		})

		It("should find the seed cluster with the minimal distance 1) referencing the same profile 2) different region 3) indicating availability 4) multiple seeds existing", func() {
			// add 3 seeds with different names and regions
			seed.Spec.Cloud.Region = "europe-north1"

			secondSeed := seedBase
			secondSeed.Name = "seed-2"
			secondSeed.Spec.Cloud.Region = "europe-west1"

			thirdSeed := seedBase
			thirdSeed.Name = "seed-3"
			thirdSeed.Spec.Cloud.Region = "asia-south1"

			gardenInformerFactory.Garden().V1beta1().Seeds().Informer().GetStore().Add(&seed)
			gardenInformerFactory.Garden().V1beta1().Seeds().Informer().GetStore().Add(&secondSeed)
			gardenInformerFactory.Garden().V1beta1().Seeds().Informer().GetStore().Add(&thirdSeed)

			// define shoot to be lexicographically 'closer' to the second seed
			anotherRegion := "europe-west3"
			shoot.Spec.Cloud.Region = anotherRegion

			bestSeed, err := determineSeed(&shoot, gardenInformerFactory.Garden().V1beta1().Seeds().Lister(), gardenInformerFactory.Garden().V1beta1().Shoots().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(secondSeed.Name))
			// verify that shoot is in another region than the chosen seed
			Expect(shoot.Spec.Cloud.Region).NotTo(Equal(bestSeed.Spec.Cloud.Region))
		})

		It("should find the best seed cluster 1) referencing the same profile 2) same  region 3) indicating availability 4) multiple seeds existing", func() {
			secondSeed := seedBase
			secondSeed.Name = "seed-2"

			gardenInformerFactory.Garden().V1beta1().Seeds().Informer().GetStore().Add(&seed)
			gardenInformerFactory.Garden().V1beta1().Seeds().Informer().GetStore().Add(&secondSeed)

			secondShoot := shootBase
			secondShoot.Name = "shoot-2"
			// first seed references more shoots then seed-2 -> expect seed-2 to be selected
			secondShoot.Spec.Cloud.Seed = &seed.Name

			gardenInformerFactory.Garden().V1beta1().Shoots().Informer().GetStore().Add(&secondShoot)

			bestSeed, err := determineSeed(&shoot, gardenInformerFactory.Garden().V1beta1().Seeds().Lister(), gardenInformerFactory.Garden().V1beta1().Shoots().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(secondSeed.Name))
		})
	})

	Context("SEED DETERMINATION - Shoot does not reference a Seed - find an adequate one using default seed determination strategy", func() {
		BeforeEach(func() {
			seed = *seedBase.DeepCopy()
			shoot = *shootBase.DeepCopy()
			schedulerConfiguration = *schedulerConfigurationBase.DeepCopy()
			gardenInformerFactory = gardeninformers.NewSharedInformerFactory(nil, 0)
			// no seed referenced
			shoot.Spec.Cloud.Seed = nil
			schedulerConfiguration.Schedulers.Shoot.Strategy = config.Default
		})

		It("should find a seed cluster 1) referencing the same profile 2) same  region 3) indicating availability", func() {
			gardenInformerFactory.Garden().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			bestSeed, err := determineSeed(&shoot, gardenInformerFactory.Garden().V1beta1().Seeds().Lister(), gardenInformerFactory.Garden().V1beta1().Shoots().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(seedName))
		})

		It("should find the best seed cluster 1) referencing the same profile 2) same  region 3) indicating availability", func() {
			secondSeed := seedBase
			secondSeed.Name = "seed-2"

			gardenInformerFactory.Garden().V1beta1().Seeds().Informer().GetStore().Add(&seed)
			gardenInformerFactory.Garden().V1beta1().Seeds().Informer().GetStore().Add(&secondSeed)

			secondShoot := shootBase
			secondShoot.Name = "shoot-2"
			secondShoot.Spec.Cloud.Seed = &seed.Name

			gardenInformerFactory.Garden().V1beta1().Shoots().Informer().GetStore().Add(&secondShoot)

			bestSeed, err := determineSeed(&shoot, gardenInformerFactory.Garden().V1beta1().Seeds().Lister(), gardenInformerFactory.Garden().V1beta1().Shoots().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).NotTo(HaveOccurred())
			Expect(bestSeed.Name).To(Equal(secondSeed.Name))
		})

		// FAIL

		It("should fail because it cannot find a seed cluster due to network disjointedness", func() {
			shoot.Spec.Cloud.AWS.Networks.K8SNetworks = gardencorev1alpha1.K8SNetworks{
				Pods:     &seed.Spec.Networks.Pods,
				Services: &seed.Spec.Networks.Services,
				Nodes:    &seed.Spec.Networks.Nodes,
			}

			gardenInformerFactory.Garden().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			bestSeed, err := determineSeed(&shoot, gardenInformerFactory.Garden().V1beta1().Seeds().Lister(), gardenInformerFactory.Garden().V1beta1().Shoots().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster due to region that no seed supports", func() {
			shoot.Spec.Cloud.Region = "another-region"

			gardenInformerFactory.Garden().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			bestSeed, err := determineSeed(&shoot, gardenInformerFactory.Garden().V1beta1().Seeds().Lister(), gardenInformerFactory.Garden().V1beta1().Shoots().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster due to invalid profile", func() {
			shoot.Spec.Cloud.Profile = "another-profile"

			gardenInformerFactory.Garden().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			bestSeed, err := determineSeed(&shoot, gardenInformerFactory.Garden().V1beta1().Seeds().Lister(), gardenInformerFactory.Garden().V1beta1().Shoots().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster due to unavailability", func() {
			seed.Status.Conditions = []gardencorev1alpha1.Condition{
				{
					Type:   gardenv1beta1.SeedAvailable,
					Status: gardencorev1alpha1.ConditionFalse,
				},
			}

			gardenInformerFactory.Garden().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			bestSeed, err := determineSeed(&shoot, gardenInformerFactory.Garden().V1beta1().Seeds().Lister(), gardenInformerFactory.Garden().V1beta1().Shoots().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

			Expect(err).To(HaveOccurred())
			Expect(bestSeed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster due to invisibility", func() {
			seed.Spec.Visible = &falseVar

			gardenInformerFactory.Garden().V1beta1().Seeds().Informer().GetStore().Add(&seed)

			bestSeed, err := determineSeed(&shoot, gardenInformerFactory.Garden().V1beta1().Seeds().Lister(), gardenInformerFactory.Garden().V1beta1().Shoots().Lister(), schedulerConfiguration.Schedulers.Shoot.Strategy)

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

			shoot.Spec.Cloud.Seed = &seed.Name
			runtimeClient.EXPECT().Update(context.TODO(), shoot).DoAndReturn(func(ctx context.Context, list runtime.Object) error {
				return nil
			})

			executeSchedulingRequest := func(ctx context.Context, shoot *gardenv1beta1.Shoot) error {
				return runtimeClient.Update(ctx, shoot)
			}

			err := UpdateShootToBeScheduledOntoSeed(context.TODO(), shoot, &seed, executeSchedulingRequest)

			Expect(err).NotTo(HaveOccurred())
		})
	})
})

func makeCIDRPtr(cidr string) *gardencorev1alpha1.CIDR {
	c := gardencorev1alpha1.CIDR(cidr)
	return &c
}
