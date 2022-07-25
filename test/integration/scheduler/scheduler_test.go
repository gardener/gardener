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
)

var _ = Describe("Scheduler tests", func() {
	Context("SameRegion Scheduling Strategy", func() {
		BeforeEach(func() {
			createAndStartManager(&config.ShootSchedulerConfiguration{ConcurrentSyncs: 1, Strategy: config.SameRegion})
		})

		It("should fail because no Seed in same region exist", func() {
			cloudProfile := createCloudProfile(providerType, "other-region")
			createSeed(providerType, "some-region")
			shoot := createShoot(providerType, cloudProfile.Name, "other-region", pointer.String("somedns.example.com"))

			Consistently(func() *string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Spec.SeedName
			}).Should(BeNil())
		})

		It("should pass because Seed and Shoot in the same region", func() {
			cloudProfile := createCloudProfile(providerType, "some-region")
			seed := createSeed(providerType, "some-region")
			shoot := createShoot(providerType, cloudProfile.Name, "some-region", pointer.String("somedns.example.com"))

			Eventually(func() *string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Spec.SeedName
			}).Should(PointTo(Equal(seed.Name)))
		})
	})

	Context("MinimalDistance Scheduling Strategy", func() {
		BeforeEach(func() {
			createAndStartManager(&config.ShootSchedulerConfiguration{ConcurrentSyncs: 1, Strategy: config.MinimalDistance})
		})

		It("should successfully schedule to Seed in region with minimal distance", func() {
			cloudProfile := createCloudProfile(providerType, "eu-west-1")

			createSeed(providerType, "us-east-1")
			createSeed(providerType, "ca-central-1")
			seed3 := createSeed(providerType, "eu-east-1")
			createSeed(providerType, "ap-west-1")
			createSeed(providerType, "us-central-2")

			shoot := createShoot(providerType, cloudProfile.Name, "eu-west-1", pointer.String("somedns.example.com"))

			Eventually(func() *string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Spec.SeedName
			}).Should(PointTo(Equal(seed3.Name)))
		})

		It("should successfully schedule to Seed in region with minimal distance", func() {
			cloudProfile := createCloudProfile(providerType, "eu-west-1")

			createSeed(providerType, "us-east-1")
			createSeed(providerType, "ca-west-2")
			seed3 := createSeed(providerType, "eu-north-1")
			createSeed(providerType, "ap-south-2")
			seed5 := createSeed(providerType, "eu-central-1")

			shoot := createShoot(providerType, cloudProfile.Name, "eu-west-1", pointer.String("somedns.example.com"))

			Eventually(func() *string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Spec.SeedName
			}).Should(PointTo(Or(Equal(seed3.Name), Equal(seed5.Name))))
		})

		It("should successfully schedule to Seed in region with minimal distance", func() {
			cloudProfile := createCloudProfile(providerType, "us-east-2")

			seed1 := createSeed(providerType, "eu-east-2")
			createSeed(providerType, "eu-west-1")
			createSeed(providerType, "ap-east-1")
			createSeed(providerType, "eu-central-2")
			createSeed(providerType, "sa-east-1")

			shoot := createShoot(providerType, cloudProfile.Name, "us-east-2", pointer.String("somedns.example.com"))

			Eventually(func() *string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Spec.SeedName
			}).Should(PointTo(Equal(seed1.Name)))
		})
	})
})

func createAndStartManager(config *config.ShootSchedulerConfiguration) {
	By("setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:             kubernetes.GardenScheme,
		MetricsBindAddress: "0",
		Namespace:          testNamespace.Name,
	})
	Expect(err).ToNot(HaveOccurred())

	By("registering controller")
	Expect(shootcontroller.AddToManager(mgr, versionedTestClient, config)).To(Succeed())

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
}

func createSeed(providerType, region string) *gardencorev1beta1.Seed {
	By("creating Seed")
	seed := &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: testID + "-",
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
	Expect(testClient.Create(ctx, seed)).To(Succeed())
	log.Info("Created Seed for test", "seed", client.ObjectKeyFromObject(seed))

	DeferCleanup(func() {
		By("deleting Seed")
		Expect(client.IgnoreNotFound(testClient.Delete(ctx, seed))).To(Succeed())
	})

	seed.Status = gardencorev1beta1.SeedStatus{
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
	Expect(testClient.Status().Update(ctx, seed)).To(Succeed())
	return seed
}

func createCloudProfile(providerType, region string) *gardencorev1beta1.CloudProfile {
	By("creating CloudProfile")
	cloudProfile := &gardencorev1beta1.CloudProfile{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: testID + "-",
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
	Expect(testClient.Create(ctx, cloudProfile)).To(Succeed())
	log.Info("Created CloudProfile for test", "cloudProfile", client.ObjectKeyFromObject(cloudProfile))

	DeferCleanup(func() {
		By("deleting CloudProfile")
		Expect(client.IgnoreNotFound(testClient.Delete(ctx, cloudProfile))).To(Succeed())
	})

	return cloudProfile
}

func createShoot(providerType, cloudProfile, region string, dnsDomain *string) *gardencorev1beta1.Shoot {
	By("creating Shoot")
	shoot := &gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
			Namespace:    testNamespace.Name,
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
			SecretBindingName: testSecretBinging.Name,
			DNS:               &gardencorev1beta1.DNS{Domain: dnsDomain},
		},
	}
	Expect(testClient.Create(ctx, shoot)).To(Succeed())
	log.Info("Created Shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

	DeferCleanup(func() {
		By("deleting Shoot")
		Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot))).To(Succeed())
	})
	return shoot
}
