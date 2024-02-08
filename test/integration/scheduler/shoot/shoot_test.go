// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/scheduler/apis/config"
	shootcontroller "github.com/gardener/gardener/pkg/scheduler/controller/shoot"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Scheduler tests", func() {
	Context("SameRegion Scheduling Strategy", func() {
		BeforeEach(func() {
			createAndStartManager(&config.ShootSchedulerConfiguration{ConcurrentSyncs: 1, Strategy: config.SameRegion})
		})

		It("should fail because no Seed in same region exist", func() {
			cloudProfile := createCloudProfile("other-region")
			createSeed("some-region", nil)
			shoot := createShoot(cloudProfile.Name, "other-region", nil, ptr.To("somedns.example.com"), nil)

			Consistently(func() *string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Spec.SeedName
			}).Should(BeNil())

			Eventually(func(g Gomega) string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				Expect(shoot.Status.LastOperation).NotTo(BeNil())
				Expect(shoot.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeCreate))
				Expect(shoot.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStatePending))
				return shoot.Status.LastOperation.Description
			}).Should(ContainSubstring("no matching seed candidate found"))
		})

		It("should fail because shoot doesn't configure the default scheduler", func() {
			cloudProfile := createCloudProfile("some-region")
			_ = createSeed("some-region", nil)
			shoot := createShoot(cloudProfile.Name, "some-region", ptr.To("foo-scheduler"), ptr.To("somedns.example.com"), nil)

			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				g.Expect(shoot.Spec.SeedName).To(BeNil())
				g.Expect(shoot.Status.LastOperation).To(BeNil())
			}).Should(Succeed())
		})

		It("should pass because Seed and Shoot in the same region", func() {
			cloudProfile := createCloudProfile("some-region")
			seed := createSeed("some-region", nil)
			shoot := createShoot(cloudProfile.Name, "some-region", nil, ptr.To("somedns.example.com"), nil)

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				g.Expect(shoot.Spec.SeedName).To(PointTo(Equal(seed.Name)))
				g.Expect(shoot.Status.LastOperation).To(BeNil())
			}).Should(Succeed())
		})

		It("should pass because there is a seed with < 3 zones for non-HA shoot", func() {
			cloudProfile := createCloudProfile("some-region")
			seed := createSeed("some-region", []string{"1"})
			shoot := createShoot(cloudProfile.Name, "some-region", nil, ptr.To("somedns.example.com"), nil)

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				g.Expect(shoot.Spec.SeedName).To(PointTo(Equal(seed.Name)))
				g.Expect(shoot.Status.LastOperation).To(BeNil())
			}).Should(Succeed())
		})

		It("should pass because there is a seed with >= 3 zones for non-HA shoot", func() {
			cloudProfile := createCloudProfile("some-region")
			seed := createSeed("some-region", []string{"1", "2", "3"})
			shoot := createShoot(cloudProfile.Name, "some-region", nil, ptr.To("somedns.example.com"), nil)

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				g.Expect(shoot.Spec.SeedName).To(PointTo(Equal(seed.Name)))
				g.Expect(shoot.Status.LastOperation).To(BeNil())
			}).Should(Succeed())
		})

		It("should pass because there is a seed with < 3 zones for shoot with failure tolerance type 'node'", func() {
			cloudProfile := createCloudProfile("some-region")
			seed := createSeed("some-region", []string{"1", "2"})
			shoot := createShoot(cloudProfile.Name, "some-region", nil, ptr.To("somedns.example.com"), getControlPlaneWithType("node"))

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				g.Expect(shoot.Spec.SeedName).To(PointTo(Equal(seed.Name)))
				g.Expect(shoot.Status.LastOperation).To(BeNil())
			}).Should(Succeed())
		})

		It("should pass because there is a seed with >= 3 zones for shoot with failure tolerance type 'node'", func() {
			cloudProfile := createCloudProfile("some-region")
			seed := createSeed("some-region", []string{"1", "2", "3"})
			shoot := createShoot(cloudProfile.Name, "some-region", nil, ptr.To("somedns.example.com"), getControlPlaneWithType("node"))

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				g.Expect(shoot.Spec.SeedName).To(PointTo(Equal(seed.Name)))
				g.Expect(shoot.Status.LastOperation).To(BeNil())
			}).Should(Succeed())
		})

		It("should fail because there is no seed with >= 3 zones for shoot with failure tolerance type 'zone'", func() {
			cloudProfile := createCloudProfile("some-region")
			_ = createSeed("some-region", []string{"1", "2"})
			shoot := createShoot(cloudProfile.Name, "some-region", nil, ptr.To("somedns.example.com"), getControlPlaneWithType("zone"))

			Consistently(func() *string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Spec.SeedName
			}).Should(BeNil())

			Eventually(func(g Gomega) string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				Expect(shoot.Status.LastOperation).NotTo(BeNil())
				Expect(shoot.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeCreate))
				Expect(shoot.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStatePending))
				return shoot.Status.LastOperation.Description
			}).Should(ContainSubstring("none of the 1 seeds has at least 3 zones for hosting a shoot control plane with failure tolerance type 'zone'"))
		})

		It("should pass because there is a seed with >= 3 zones for shoot with failure tolerance type 'zone'", func() {
			cloudProfile := createCloudProfile("some-region")
			seed := createSeed("some-region", []string{"1", "2", "3"})
			shoot := createShoot(cloudProfile.Name, "some-region", nil, ptr.To("somedns.example.com"), getControlPlaneWithType("zone"))

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				g.Expect(shoot.Spec.SeedName).To(PointTo(Equal(seed.Name)))
				g.Expect(shoot.Status.LastOperation).To(BeNil())
			}).Should(Succeed())
		})
	})

	Context("MinimalDistance Scheduling Strategy", func() {
		var seedAPWest1, seedEUEast1, seedEUWest1, seedUSCentral2, seedUSEast1 *gardencorev1beta1.Seed

		BeforeEach(func() {
			createAndStartManager(&config.ShootSchedulerConfiguration{ConcurrentSyncs: 1, Strategy: config.MinimalDistance})

			seedAPWest1 = createSeed("ap-west-1", nil)
			seedEUEast1 = createSeed("eu-east-1", nil)
			seedEUWest1 = createSeed("eu-west-1", nil)
			seedUSCentral2 = createSeed("us-central-2", nil)
			seedUSEast1 = createSeed("us-east-1", nil)
		})

		Context("with region config", func() {
			AfterEach(func() {
				configMapList := &corev1.ConfigMapList{}
				Expect(testClient.List(ctx, configMapList, client.MatchingLabels{"scheduling.gardener.cloud/purpose": "region-config"})).To(Succeed())

				for _, configMap := range configMapList.Items {
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, &configMap))).NotTo(HaveOccurred(), fmt.Sprintf("deleting ConfigMap %s", client.ObjectKeyFromObject(&configMap)))
				}
			})

			It("should successfully schedule to closest seed in the same region", func() {
				cloudProfile := createCloudProfile("eu-west-1")

				regionConfig := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cloudProfile.Name,
						Namespace: testNamespace.Name,
						Labels: map[string]string{
							"scheduling.gardener.cloud/purpose": "region-config",
						},
						Annotations: map[string]string{
							"scheduling.gardener.cloud/cloudprofiles": cloudProfile.Name,
						},
					},

					Data: map[string]string{
						"eu-west-1": `
us-east-1: 20
eu-east-1: 50
ap-west-1: 300
us-central-2: 220`,
					},
				}
				Expect(testClient.Create(ctx, regionConfig)).To(Succeed())

				By("Wait until manager has observed region config")
				// Use the manager's cache to ensure it has observed the configMap.
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(regionConfig), &corev1.ConfigMap{})
				}).Should(Succeed())

				shoot := createShoot(cloudProfile.Name, "eu-west-1", nil, ptr.To("somedns.example.com"), nil)

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Spec.SeedName).To(PointTo(Equal(seedEUWest1.Name)))
					g.Expect(shoot.Status.LastOperation).To(BeNil())
				}).Should(Succeed())
			})

			It("should successfully schedule to closest seed in a different region", func() {
				cloudProfile := createCloudProfile("eu-west-1")

				regionConfig := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cloudProfile.Name,
						Namespace: testNamespace.Name,
						Labels: map[string]string{
							"scheduling.gardener.cloud/purpose": "region-config",
						},
						Annotations: map[string]string{
							"scheduling.gardener.cloud/cloudprofiles": cloudProfile.Name,
						},
					},
					// Choose a better value for 'us-east-1' than for 'eu-west-1' to test that the minimal configured distance is really used, not Levenshtein's algorithm.
					// Also, the distance to itself is higher than other values, so that the logic prefers other regions.
					Data: map[string]string{
						"eu-west-1": `
eu-west-1: 30
us-east-1: 20
eu-east-1: 50
ap-west-1: 300
us-central-2: 220`,
					},
				}
				Expect(testClient.Create(ctx, regionConfig)).To(Succeed())

				By("Wait until manager has observed region config")
				// Use the manager's cache to ensure it has observed the configMap.
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(regionConfig), &corev1.ConfigMap{})
				}).Should(Succeed())

				shoot := createShoot(cloudProfile.Name, "eu-west-1", nil, ptr.To("somedns.example.com"), nil)

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Spec.SeedName).To(PointTo(Equal(seedUSEast1.Name)))
					g.Expect(shoot.Status.LastOperation).To(BeNil())
				}).Should(Succeed())
			})

			It("should successfully schedule to closest seed if multiple configs are found", func() {
				cloudProfile := createCloudProfile("eu-west-1")

				regionConfig1 := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "a",
						Namespace: testNamespace.Name,
						Labels: map[string]string{
							"scheduling.gardener.cloud/purpose": "region-config",
						},
						Annotations: map[string]string{
							"scheduling.gardener.cloud/cloudprofiles": "foo-cloudprofile," + cloudProfile.Name,
						},
					},

					Data: map[string]string{
						"eu-west-1": `
us-east-1: 20
eu-east-1: 50
ap-west-1: 300
us-central-2: 220`,
					},
				}

				regionConfig2 := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "b",
						Namespace: testNamespace.Name,
						Labels: map[string]string{
							"scheduling.gardener.cloud/purpose": "region-config",
						},
						Annotations: map[string]string{
							"scheduling.gardener.cloud/cloudprofiles": cloudProfile.Name,
						},
					},

					Data: map[string]string{
						"eu-west-1": `
eu-west-1: 30
us-east-1: 40
eu-east-1: 20
ap-west-1: 300
us-central-2: 220`,
					},
				}
				Expect(testClient.Create(ctx, regionConfig1)).To(Succeed())
				Expect(testClient.Create(ctx, regionConfig2)).To(Succeed())

				By("Wait until manager has observed region config")
				// Use the manager's cache to ensure it has observed the configMap.
				Eventually(func() ([]corev1.ConfigMap, error) {
					configMapList := &corev1.ConfigMapList{}
					if err := testClient.List(ctx, configMapList, client.HasLabels{"scheduling.gardener.cloud/purpose"}); err != nil {
						return nil, err
					}
					return configMapList.Items, nil
				}).Should(HaveLen(2))

				shoot := createShoot(cloudProfile.Name, "eu-west-1", nil, ptr.To("somedns.example.com"), nil)

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Spec.SeedName).To(PointTo(Or(Equal(seedEUWest1.Name), Equal(seedEUEast1.Name))))
					g.Expect(shoot.Status.LastOperation).To(BeNil())
				}).Should(Succeed())
			})

			It("should fall back to Levenshtein minimal distance if shoot region is not configured", func() {
				cloudProfile := createCloudProfile("eu-west-1")

				regionConfig := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cloudProfile.Name,
						Namespace: testNamespace.Name,
						Labels: map[string]string{
							"scheduling.gardener.cloud/purpose": "region-config",
						},
						Annotations: map[string]string{
							"scheduling.gardener.cloud/cloudprofiles": cloudProfile.Name,
						},
					},
					Data: map[string]string{
						"us-east-1": `
eu-west-1: 30
eu-east-1: 50
ap-west-1: 300
us-central-2: 220`,
					},
				}
				Expect(testClient.Create(ctx, regionConfig)).To(Succeed())

				By("Wait until manager has observed region config")
				// Use the manager's cache to ensure it has observed the configMap.
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(regionConfig), &corev1.ConfigMap{})
				}).Should(Succeed())

				shoot := createShoot(cloudProfile.Name, "eu-west-1", nil, ptr.To("somedns.example.com"), nil)

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Spec.SeedName).To(PointTo(Equal(seedEUWest1.Name)))
					g.Expect(shoot.Status.LastOperation).To(BeNil())
				}).Should(Succeed())
			})

			It("should fall back to Levenshtein minimal distance if seed regions are missing", func() {
				Expect(testClient.Delete(ctx, seedEUWest1)).To(Succeed())

				cloudProfile := createCloudProfile("eu-west-1")

				regionConfig := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cloudProfile.Name,
						Namespace: testNamespace.Name,
						Labels: map[string]string{
							"scheduling.gardener.cloud/purpose": "region-config",
						},
						Annotations: map[string]string{
							"scheduling.gardener.cloud/cloudprofiles": cloudProfile.Name,
						},
					},
					Data: map[string]string{
						"eu-west-1": `
eu-west-2: 30
us-east-2: 1
eu-east-2: 50
ap-west-2: 300
us-central-3: 220`,
					},
				}
				Expect(testClient.Create(ctx, regionConfig)).To(Succeed())

				By("Wait until manager has observed region config")
				// Use the manager's cache to ensure it has observed the configMap.
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(regionConfig), &corev1.ConfigMap{})
				}).Should(Succeed())

				shoot := createShoot(cloudProfile.Name, "eu-west-1", nil, ptr.To("somedns.example.com"), nil)

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Spec.SeedName).To(PointTo(Equal(seedEUEast1.Name)))
					g.Expect(shoot.Status.LastOperation).To(BeNil())
				}).Should(Succeed())
			})

			It("should fail to schedule to Seed if region config is not parseable", func() {
				cloudProfile := createCloudProfile("eu-west-1")

				regionConfig := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cloudProfile.Name,
						Namespace: testNamespace.Name,
						Labels: map[string]string{
							"scheduling.gardener.cloud/purpose": "region-config",
						},
						Annotations: map[string]string{
							"scheduling.gardener.cloud/cloudprofiles": cloudProfile.Name,
						},
					},
					Data: map[string]string{
						"eu-west-1": `
{`,
					},
				}
				Expect(testClient.Create(ctx, regionConfig)).To(Succeed())

				By("Wait until manager has observed region config")
				// Use the manager's cache to ensure it has observed the configMap.
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(regionConfig), &corev1.ConfigMap{})
				}).Should(Succeed())

				shoot := createShoot(cloudProfile.Name, "eu-west-1", nil, ptr.To("somedns.example.com"), nil)

				Consistently(func() *string {
					Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					return shoot.Spec.SeedName
				}).Should(BeNil())

				Eventually(func(g Gomega) string {
					Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					Expect(shoot.Status.LastOperation).NotTo(BeNil())
					Expect(shoot.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeCreate))
					Expect(shoot.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStatePending))
					return shoot.Status.LastOperation.Description
				}).Should(ContainSubstring("failed to determine seed candidates. Wrong format in region ConfigMap"))
			})
		})

		Context("fallback - without region config", func() {
			It("should successfully schedule to Seed in region with minimal distance (prefer own region)", func() {
				cloudProfile := createCloudProfile("eu-west-1")

				shoot := createShoot(cloudProfile.Name, "eu-west-1", nil, ptr.To("somedns.example.com"), nil)

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Spec.SeedName).To(PointTo(Equal(seedEUWest1.Name)))
					g.Expect(shoot.Status.LastOperation).To(BeNil())
				}).Should(Succeed())
			})

			It("should successfully schedule to Seed in region with minimal distance (prefer own zone)", func() {
				cloudProfile := createCloudProfile("eu-west-1")

				shoot := createShoot(cloudProfile.Name, "eu-west-1", nil, ptr.To("somedns.example.com"), nil)

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Spec.SeedName).To(PointTo(Equal(seedEUWest1.Name)))
					g.Expect(shoot.Status.LastOperation).To(BeNil())
				}).Should(Succeed())
			})

			It("should successfully schedule to Seed in region with minimal distance (prefer same continent - multiple options)", func() {
				cloudProfile := createCloudProfile("eu-central-1")

				shoot := createShoot(cloudProfile.Name, "eu-central-1", nil, ptr.To("somedns.example.com"), nil)

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Spec.SeedName).To(PointTo(Or(Equal(seedEUEast1.Name), Equal(seedEUWest1.Name))))
					g.Expect(shoot.Status.LastOperation).To(BeNil())
				}).Should(Succeed())
			})

			It("should successfully schedule to Seed minimal distance in different region", func() {
				cloudProfile := createCloudProfile("jp-west-1")

				shoot := createShoot(cloudProfile.Name, "jp-west-1", nil, ptr.To("somedns.example.com"), nil)

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Spec.SeedName).To(PointTo(Equal(seedAPWest1.Name)))
					g.Expect(shoot.Status.LastOperation).To(BeNil())
				}).Should(Succeed())
			})

			It("should successfully schedule to Seed with >= 3 zones in region with minimal distance", func() {
				patch := client.StrategicMergeFrom(seedEUWest1.DeepCopy())
				seedEUWest1.Spec.Provider.Zones = []string{"1", "2", "3"}
				Expect(testClient.Patch(ctx, seedEUWest1, patch)).To(Succeed())

				patch = client.StrategicMergeFrom(seedEUWest1.DeepCopy())
				seedUSCentral2.Spec.Provider.Zones = []string{"1", "2", "3"}
				Expect(testClient.Patch(ctx, seedEUWest1, patch)).To(Succeed())

				cloudProfile := createCloudProfile("eu-east-1")

				shoot := createShoot(cloudProfile.Name, "eu-east-1", nil, ptr.To("somedns.example.com"), getControlPlaneWithType("zone"))

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Spec.SeedName).To(PointTo(Equal(seedEUWest1.Name)))
					g.Expect(shoot.Status.LastOperation).To(BeNil())
				}).Should(Succeed())
			})
		})
	})
})

func createAndStartManager(config *config.ShootSchedulerConfiguration) {
	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  kubernetes.GardenScheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{testNamespace.Name: {}},
		},
	})
	Expect(err).NotTo(HaveOccurred())

	By("Register controller")
	Expect((&shootcontroller.Reconciler{
		Config:          config,
		GardenNamespace: testNamespace.Name,
	}).AddToManager(mgr)).To(Succeed())

	By("Start manager")
	mgrContext, mgrCancel := context.WithCancel(ctx)

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(mgrContext)).To(Succeed())
	}()

	DeferCleanup(func() {
		By("Stop manager")
		mgrCancel()
	})
}

func createSeed(region string, zones []string) *gardencorev1beta1.Seed {
	By("Create Seed")
	seed := &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: testID + "-",
		},
		Spec: gardencorev1beta1.SeedSpec{
			Provider: gardencorev1beta1.SeedProvider{
				Type:   "provider-type",
				Region: region,
				Zones:  zones,
			},
			Ingress: &gardencorev1beta1.Ingress{
				Domain: "seed.example.com",
				Controller: gardencorev1beta1.IngressController{
					Kind: "nginx",
				},
			},
			DNS: gardencorev1beta1.SeedDNS{
				Provider: &gardencorev1beta1.SeedDNSProvider{
					Type: "provider-type",
					SecretRef: corev1.SecretReference{
						Name:      "some-secret",
						Namespace: "some-namespace",
					},
				},
			},
			Settings: &gardencorev1beta1.SeedSettings{
				Scheduling: &gardencorev1beta1.SeedSettingScheduling{Visible: true},
			},
			Networks: gardencorev1beta1.SeedNetworks{
				Pods:     "10.0.0.0/16",
				Services: "10.1.0.0/16",
				Nodes:    ptr.To("10.2.0.0/16"),
			},
		},
	}
	ExpectWithOffset(1, testClient.Create(ctx, seed)).To(Succeed())
	log.Info("Created Seed for test", "seed", client.ObjectKeyFromObject(seed), "region", seed.Spec.Provider.Region)

	DeferCleanup(func() {
		By("Delete Seed")
		ExpectWithOffset(1, client.IgnoreNotFound(testClient.Delete(ctx, seed))).To(Succeed())
	})

	seed.Status = gardencorev1beta1.SeedStatus{
		Allocatable: corev1.ResourceList{
			gardencorev1beta1.ResourceShoots: resource.MustParse("100"),
		},
		Conditions: []gardencorev1beta1.Condition{
			{
				Type:   gardencorev1beta1.SeedGardenletReady,
				Status: gardencorev1beta1.ConditionTrue,
			},
		},
		LastOperation: &gardencorev1beta1.LastOperation{},
	}
	ExpectWithOffset(1, testClient.Status().Update(ctx, seed)).To(Succeed())
	return seed
}

func createCloudProfile(region string) *gardencorev1beta1.CloudProfile {
	By("Create CloudProfile")
	cloudProfile := &gardencorev1beta1.CloudProfile{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: testID + "-",
		},
		Spec: gardencorev1beta1.CloudProfileSpec{
			Kubernetes: gardencorev1beta1.KubernetesSettings{
				Versions: []gardencorev1beta1.ExpirableVersion{{Version: "1.25.1"}},
			},
			MachineImages: []gardencorev1beta1.MachineImage{
				{
					Name: "some-OS",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.1.1"},
							CRI: []gardencorev1beta1.CRI{
								{
									Name: gardencorev1beta1.CRINameContainerD,
								},
							},
						},
					},
				},
			},
			MachineTypes: []gardencorev1beta1.MachineType{{Name: "large"}},
			Regions:      []gardencorev1beta1.Region{{Name: region}},
			Type:         "provider-type",
		},
	}
	ExpectWithOffset(1, testClient.Create(ctx, cloudProfile)).To(Succeed())
	log.Info("Created CloudProfile for test", "cloudProfile", client.ObjectKeyFromObject(cloudProfile))

	DeferCleanup(func() {
		By("Delete CloudProfile")
		ExpectWithOffset(1, client.IgnoreNotFound(testClient.Delete(ctx, cloudProfile))).To(Succeed())
	})

	return cloudProfile
}

func createShoot(cloudProfile, region string, schedulerName, dnsDomain *string, controlPlane *gardencorev1beta1.ControlPlane) *gardencorev1beta1.Shoot {
	By("Create Shoot")
	shoot := &gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
			Namespace:    testNamespace.Name,
		},
		Spec: gardencorev1beta1.ShootSpec{
			ControlPlane:     controlPlane,
			CloudProfileName: cloudProfile,
			Region:           region,
			Provider: gardencorev1beta1.Provider{
				Type: "provider-type",
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
			Networking: &gardencorev1beta1.Networking{
				Pods:     ptr.To("10.3.0.0/16"),
				Services: ptr.To("10.4.0.0/16"),
				Nodes:    ptr.To("10.5.0.0/16"),
				Type:     ptr.To("some-type"),
			},
			Kubernetes:        gardencorev1beta1.Kubernetes{Version: "1.25.1"},
			SecretBindingName: ptr.To(testSecretBinding.Name),
			SchedulerName:     schedulerName,
			DNS:               &gardencorev1beta1.DNS{Domain: dnsDomain},
		},
	}
	Expect(testClient.Create(ctx, shoot)).To(Succeed())
	log.Info("Created Shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

	DeferCleanup(func() {
		By("Delete Shoot")
		Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot))).To(Succeed())
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
		}).Should(BeNotFoundError())
	})
	return shoot
}

func getHighAvailabilityWithType(failureToleranceType string) *gardencorev1beta1.HighAvailability {
	return &gardencorev1beta1.HighAvailability{
		FailureTolerance: gardencorev1beta1.FailureTolerance{
			Type: gardencorev1beta1.FailureToleranceType(failureToleranceType),
		},
	}
}

func getControlPlaneWithType(failureToleranceType string) *gardencorev1beta1.ControlPlane {
	return &gardencorev1beta1.ControlPlane{
		HighAvailability: getHighAvailabilityWithType(failureToleranceType),
	}
}
