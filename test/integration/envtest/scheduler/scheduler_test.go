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

/**
	Overview
		- Tests the Gardener Scheduler

	Prerequisites
		- The Gardener-Scheduler is running

	BeforeSuite
		- Parse valid Shoot from example folder and flags. Remove the Spec.SeedName.
		- If running in TestMachinery: Scale down the GardenerController Manager

	AfterSuite
		- Delete Shoot
        - If running in TestMachinery: Scale up the GardenerController Manager

	Test: SameRegion Scheduling Strategy Test
		1) Create Shoot in region where no Seed exists. (e.g Shoot in eu-west-1 and only Seed exists in us-east-1)
		   Expected Output
			 - should fail because no Seed in same region exists1)
	Test: Minimal Distance Scheduling Strategy Test
		1) Create Shoot in region where no Seed exists. (e.g Shoot in eu-west-1 and only Seed exists in us-east-1)
		   Expected Output
			 - should successfully schedule to Seed in region with minimal distance
	Test: Api Server ShootBindingStrategy test
		1) Request APiServer to schedule shoot to non-existing seed
		   Expected Output
			 - Error from ApiServer
		2) Request APiServer to schedule shoot that is already scheduled to another seed
		   Expected Output
			 - Error from ApiServer
 **/

package scheduler

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/scheduler/apis/config"
	shootcontroller "github.com/gardener/gardener/pkg/scheduler/controller/shoot"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Scheduler tests", func() {
	Context("Same Region Scheduling Strategy test", func() {
		var (
			seed         *gardencorev1beta1.Seed
			shoot        *gardencorev1beta1.Shoot
			cloudProfile *gardencorev1beta1.CloudProfile
			providerType = "provider-type"
		)

		BeforeEach(func() {
			currentConfig := &config.ShootSchedulerConfiguration{ConcurrentSyncs: 1, Strategy: config.SameRegion}
			mgr := createManager(currentConfig)
			mgrContext, mgrCancel = context.WithCancel(ctx)

			By("start manager")
			go func() {
				err := mgr.Start(mgrContext)
				Expect(err).ToNot(HaveOccurred())
			}()
		})

		AfterEach(func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "confirmation.gardener.cloud/deletion", "true")
			Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())
			Expect(testClient.Delete(ctx, shoot)).To(Succeed())

			Expect(testClient.Delete(ctx, cloudProfile)).To(Succeed())
			Expect(testClient.Delete(ctx, seed)).To(Succeed())

			By("Stopping Manager")
			mgrCancel()
		})

		It("should fail because no Seed in same region exist", func() {
			By("create cloudprofile")
			cloudProfile = createCloudProfile("cloudprofile", providerType, "other-region")

			By("create seed")
			seed = createSeed("seed", providerType, "some-region")

			By("create shoot")
			shoot = createShoot("shoot", providerType, cloudProfile.Name, "other-region", pointer.String("somedns.example.com"))

			Consistently(func() *string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Spec.SeedName
			}).Should(BeNil())
		})

		It("Should pass because Seed and Shoot in the same region", func() {

			By("create cloudprofile")
			cloudProfile = createCloudProfile("cloudprofile", providerType, "some-region")

			By("create seed")
			seed = createSeed("seed", providerType, "some-region")

			By("create shoot")
			DNS := pointer.String("somedns.example.com")
			shoot = createShoot("shoot", providerType, "cloudprofile", "some-region", DNS)

			Eventually(func() *string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Spec.SeedName
			}).Should(PointTo(Equal("seed")))
		})
	})

	Context("Minimal Distance Scheduling Strategy test", func() {
		var (
			ctx          = context.Background()
			seeds        [5]*gardencorev1beta1.Seed
			shoot        *gardencorev1beta1.Shoot
			cloudProfile *gardencorev1beta1.CloudProfile
			providerType = "provider-type"
		)

		BeforeEach(func() {
			currentConfig := &config.ShootSchedulerConfiguration{ConcurrentSyncs: 1, Strategy: config.MinimalDistance}
			mgr := createManager(currentConfig)
			mgrContext, mgrCancel = context.WithCancel(ctx)

			By("start manager")
			go func() {
				err := mgr.Start(mgrContext)
				Expect(err).ToNot(HaveOccurred())
			}()
		})

		AfterEach(func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "confirmation.gardener.cloud/deletion", "true")
			Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())
			Expect(testClient.Delete(ctx, shoot)).To(Succeed())

			Expect(testClient.Delete(ctx, cloudProfile)).To(Succeed())

			for i := range seeds {
				Expect(testClient.Delete(ctx, seeds[i])).To(Succeed())
			}
			By("Stopping Manager")
			mgrCancel()
		})

		It("Should successfully schedule to Seed in region with minimal distance", func() {

			By("create cloudprofile")
			cloudProfile = createCloudProfile("cloudprofile", providerType, "eu-west-1")

			By("create seed")
			seeds[0] = createSeed("seed1", providerType, "us-east-1")
			seeds[1] = createSeed("seed2", providerType, "ca-central-1")
			seeds[2] = createSeed("seed3", providerType, "eu-east-1")
			seeds[3] = createSeed("seed4", providerType, "ap-west-1")
			seeds[4] = createSeed("seed5", providerType, "us-central-2")

			By("create shoot")
			DNS := pointer.String("somedns.example.com")
			shoot = createShoot("shoot", providerType, "cloudprofile", "eu-west-1", DNS)

			Eventually(func() *string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Spec.SeedName
			}).Should(PointTo(Equal("seed3")))
		})

		It("Should successfully schedule to Seed in region with minimal distance", func() {

			By("create cloudprofile")
			cloudProfile = createCloudProfile("cloudprofile", providerType, "eu-west-1")

			By("create seed")
			seeds[0] = createSeed("seed1", providerType, "us-east-1")
			seeds[1] = createSeed("seed2", providerType, "ca-west-2")
			seeds[2] = createSeed("seed3", providerType, "eu-north-1")
			seeds[3] = createSeed("seed4", providerType, "ap-south-2")
			seeds[4] = createSeed("seed5", providerType, "eu-central-1")

			By("create shoot")
			DNS := pointer.String("somedns.example.com")
			shoot = createShoot("shoot", providerType, "cloudprofile", "eu-west-1", DNS)

			Eventually(func() *string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Spec.SeedName
			}).Should(PointTo(Or(Equal("seed3"), Equal("seed5"))))
		})

		It("Should successfully schedule to Seed in region with minimal distance", func() {

			By("create cloudprofile")
			cloudProfile = createCloudProfile("cloudprofile", providerType, "us-east-2")

			By("create seed")
			seeds[0] = createSeed("seed1", providerType, "eu-east-2")
			seeds[1] = createSeed("seed2", providerType, "eu-west-1")
			seeds[2] = createSeed("seed3", providerType, "ap-east-1")
			seeds[3] = createSeed("seed4", providerType, "eu-central-2")
			seeds[4] = createSeed("seed5", providerType, "sa-east-1")

			By("create shoot")
			DNS := pointer.String("somedns.example.com")
			shoot = createShoot("shoot", providerType, "cloudprofile", "us-east-2", DNS)

			Eventually(func() *string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Spec.SeedName
			}).Should(PointTo(Equal("seed1")))
		})
	})
})

func createManager(config *config.ShootSchedulerConfiguration) manager.Manager {
	By("setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:             kubernetes.GardenScheme,
		MetricsBindAddress: "0",
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(shootcontroller.AddToManager(mgr, config)).To(Succeed())
	return mgr
}

func createSeed(seedName string, providerType string, region string) *gardencorev1beta1.Seed {
	obj := &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name: seedName,
		},
		Spec: gardencorev1beta1.SeedSpec{
			Provider: gardencorev1beta1.SeedProvider{
				Region: region,
				Type:   providerType,
			},
			Settings: &gardencorev1beta1.SeedSettings{
				ShootDNS:   &gardencorev1beta1.SeedSettingShootDNS{Enabled: true},
				Scheduling: &gardencorev1beta1.SeedSettingScheduling{Visible: true},
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
	Expect(testClient.Create(ctx, obj)).To(Succeed())

	obj.Status = gardencorev1beta1.SeedStatus{
		Allocatable: corev1.ResourceList{
			gardencorev1beta1.ResourceShoots: resource.MustParse("100"),
		},
		Conditions: []gardencorev1beta1.Condition{
			{
				Type:   gardencorev1beta1.SeedBootstrapped,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   gardencorev1beta1.SeedGardenletReady,
				Status: gardencorev1beta1.ConditionTrue,
			},
		},
	}
	Expect(testClient.Status().Update(ctx, obj)).To(Succeed())
	return obj
}

func createCloudProfile(cloudProfileName string, providerType string, region string) *gardencorev1beta1.CloudProfile {
	obj := &gardencorev1beta1.CloudProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: cloudProfileName,
		},
		Spec: gardencorev1beta1.CloudProfileSpec{
			Kubernetes: gardencorev1beta1.KubernetesSettings{
				Versions: []gardencorev1beta1.ExpirableVersion{{Version: "1.21.1"}},
			},
			MachineImages: []gardencorev1beta1.MachineImage{
				{
					Name: "some-OS",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.1.1"},
							CRI:              []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
						},
					},
				},
			},
			MachineTypes: []gardencorev1beta1.MachineType{{Name: "large"}},
			Regions:      []gardencorev1beta1.Region{{Name: region}},
			Type:         providerType,
		},
	}
	Expect(testClient.Create(ctx, obj)).To(Succeed())
	return obj
}

func createShoot(shootName string, providerType string, cloudProfile string, region string, DNS *string) *gardencorev1beta1.Shoot {
	obj := &gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shootName,
			Namespace: "garden-dev",
		},
		Spec: gardencorev1beta1.ShootSpec{
			CloudProfileName: cloudProfile,
			Region:           region,
			Provider: gardencorev1beta1.Provider{
				Type: providerType,
				Workers: []gardencorev1beta1.Worker{
					{
						Name:             "worker1",
						SystemComponents: &gardencorev1beta1.WorkerSystemComponents{Allow: true},
						Minimum:          1,
						Maximum:          1,
						Machine: gardencorev1beta1.Machine{
							Type:  "large",
							Image: &gardencorev1beta1.ShootMachineImage{Name: "some-OS"},
						},
					},
				},
			},
			Networking: gardencorev1beta1.Networking{
				Pods:     pointer.String("10.3.0.0/16"),
				Services: pointer.String("10.4.0.0/16"),
				Nodes:    pointer.String("10.5.0.0/16"),
				Type:     "some-type",
			},
			Kubernetes:        gardencorev1beta1.Kubernetes{Version: "1.21.1"},
			SecretBindingName: "secret",
			DNS: &gardencorev1beta1.DNS{
				Domain: DNS,
			},
		},
	}
	Expect(testClient.Create(ctx, obj)).To(Succeed())
	return obj
}
