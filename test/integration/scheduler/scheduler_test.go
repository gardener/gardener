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

package scheduler

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/scheduler/apis/config"
	shootcontroller "github.com/gardener/gardener/pkg/scheduler/controller/shoot"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Scheduler tests", func() {
	var (
		seeds        []*gardencorev1beta1.Seed
		shoot        *gardencorev1beta1.Shoot
		cloudProfile *gardencorev1beta1.CloudProfile
		providerType = "provider-type"
	)
	AfterEach(func() {
		Expect(ConfirmDeletion(ctx, testClient, shoot)).To(Succeed())
		Expect(testClient.Delete(ctx, shoot)).To(Succeed())
		Expect(testClient.Delete(ctx, cloudProfile)).To(Succeed())

		for _, seed := range seeds {
			Expect(testClient.Delete(ctx, seed)).To(Succeed())
		}
		seeds = nil
		cloudProfile = nil
		shoot = nil

		By("Stopping Manager")
		mgrCancel()
	})
	Context("SameRegion Scheduling Strategy", func() {
		BeforeEach(func() {
			mgr := createManager(&config.ShootSchedulerConfiguration{ConcurrentSyncs: 1, Strategy: config.SameRegion})
			mgrContext, mgrCancel = context.WithCancel(ctx)

			By("start manager")
			go func() {
				defer GinkgoRecover()
				Expect(mgr.Start(mgrContext)).To(Succeed())
			}()
		})

		It("should fail because no Seed in same region exist", func() {
			By("create cloudprofile")
			cloudProfile = createCloudProfile("cloudprofile", providerType, "other-region")

			By("create seed")
			seed := createSeed("seed", providerType, "some-region")
			seeds = append(seeds, seed)

			By("create shoot")
			shoot = createShoot("shoot", providerType, cloudProfile.Name, "other-region", pointer.String("somedns.example.com"))

			Consistently(func() *string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Spec.SeedName
			}).Should(BeNil())
		})

		It("should pass because Seed and Shoot in the same region", func() {
			By("create cloudprofile")
			cloudProfile = createCloudProfile("cloudprofile", providerType, "some-region")

			By("create seed")
			seed := createSeed("seed", providerType, "some-region")
			seeds = append(seeds, seed)

			By("create shoot")
			shoot = createShoot("shoot", providerType, cloudProfile.Name, "some-region", pointer.String("somedns.example.com"))

			Eventually(func() *string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Spec.SeedName
			}).Should(PointTo(Equal(seed.Name)))
		})
	})

	Context("MinimalDistance Scheduling Strategy", func() {
		BeforeEach(func() {
			mgr := createManager(&config.ShootSchedulerConfiguration{ConcurrentSyncs: 1, Strategy: config.MinimalDistance})
			mgrContext, mgrCancel = context.WithCancel(ctx)

			By("start manager")
			go func() {
				defer GinkgoRecover()
				Expect(mgr.Start(mgrContext)).To(Succeed())
			}()
		})

		It("should successfully schedule to Seed in region with minimal distance", func() {
			By("create cloudprofile")
			cloudProfile = createCloudProfile("cloudprofile", providerType, "eu-west-1")

			By("create seed")
			seed1 := createSeed("seed1", providerType, "us-east-1")
			seed2 := createSeed("seed2", providerType, "ca-central-1")
			seed3 := createSeed("seed3", providerType, "eu-east-1")
			seed4 := createSeed("seed4", providerType, "ap-west-1")
			seed5 := createSeed("seed5", providerType, "us-central-2")
			seeds = append(seeds, seed1, seed2, seed3, seed4, seed5)

			By("create shoot")
			shoot = createShoot("shoot", providerType, cloudProfile.Name, "eu-west-1", pointer.String("somedns.example.com"))

			Eventually(func() *string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Spec.SeedName
			}).Should(PointTo(Equal(seed3.Name)))
		})

		It("should successfully schedule to Seed in region with minimal distance", func() {
			By("create cloudprofile")
			cloudProfile = createCloudProfile("cloudprofile", providerType, "eu-west-1")

			By("create seed")
			seed1 := createSeed("seed1", providerType, "us-east-1")
			seed2 := createSeed("seed2", providerType, "ca-west-2")
			seed3 := createSeed("seed3", providerType, "eu-north-1")
			seed4 := createSeed("seed4", providerType, "ap-south-2")
			seed5 := createSeed("seed5", providerType, "eu-central-1")
			seeds = append(seeds, seed1, seed2, seed3, seed4, seed5)

			By("create shoot")
			shoot = createShoot("shoot", providerType, cloudProfile.Name, "eu-west-1", pointer.String("somedns.example.com"))

			Eventually(func() *string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Spec.SeedName
			}).Should(PointTo(Or(Equal(seed3.Name), Equal(seed5.Name))))
		})

		It("should successfully schedule to Seed in region with minimal distance", func() {
			By("create cloudprofile")
			cloudProfile = createCloudProfile("cloudprofile", providerType, "us-east-2")

			By("create seed")
			seed1 := createSeed("seed1", providerType, "eu-east-2")
			seed2 := createSeed("seed2", providerType, "eu-west-1")
			seed3 := createSeed("seed3", providerType, "ap-east-1")
			seed4 := createSeed("seed4", providerType, "eu-central-2")
			seed5 := createSeed("seed5", providerType, "sa-east-1")
			seeds = append(seeds, seed1, seed2, seed3, seed4, seed5)

			By("create shoot")
			shoot = createShoot("shoot", providerType, cloudProfile.Name, "us-east-2", pointer.String("somedns.example.com"))

			Eventually(func() *string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Spec.SeedName
			}).Should(PointTo(Equal(seed1.Name)))
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

func createSeed(seedName, providerType, region string) *gardencorev1beta1.Seed {
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

func createCloudProfile(cloudProfileName, providerType, region string) *gardencorev1beta1.CloudProfile {
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

func createShoot(shootName, providerType, cloudProfile, region string, dnsDomain *string) *gardencorev1beta1.Shoot {
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
			DNS:               &gardencorev1beta1.DNS{Domain: dnsDomain},
		},
	}
	Expect(testClient.Create(ctx, obj)).To(Succeed())
	return obj
}
