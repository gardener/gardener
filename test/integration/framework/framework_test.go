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

package framework_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/test/integration/framework"
)

func TestFramework(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Framework Test Suite")
}

var _ = Describe("Framework tests", func() {

	Context("Download Chart Artifacts", func() {
		var (
			resourcesDir = filepath.Join("..", "resources")
			chartRepo    = filepath.Join(resourcesDir, "charts")
			helm         = Helm(resourcesDir)
		)

		AfterEach(func() {
			err := os.RemoveAll(filepath.Join(resourcesDir, "charts", "redis"))
			Expect(err).NotTo(HaveOccurred())

			err = os.RemoveAll(filepath.Join(resourcesDir, "repository", "cache"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should download chart artifacts", func() {
			shootTestOperation := GardenerTestOperation{
				Logger: logger.AddWriter(logger.NewLogger("info"), GinkgoWriter),
			}

			err := shootTestOperation.DownloadChartArtifacts(context.TODO(), helm, chartRepo, "stable/redis", "7.0.0")
			Expect(err).NotTo(HaveOccurred())

			expectedCachePath := filepath.Join(resourcesDir, "repository", "cache", "stable-index.yaml")
			cacheIndexExists, err := Exists(expectedCachePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(cacheIndexExists).To(BeTrue())

			expectedRedisChartPath := filepath.Join(resourcesDir, "charts", "redis")
			chartExists, err := Exists(expectedRedisChartPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(chartExists).To(BeTrue())
		})
	})

	Context("Scheduler Operations - ChooseRegionAndZoneWithNoSeed", func() {
		var (
			cloudProfile     gardenv1beta1.CloudProfile
			seed             gardenv1beta1.Seed
			allSeeds         []gardenv1beta1.Seed
			cloudProfileName = "cloudprofile-1"
			seedName         = "seed-1"
			regionEuropeWest = "europe-west1"
			regionEuropeEast = "us-east1"

			seedBase = gardenv1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
				Spec: gardenv1beta1.SeedSpec{
					Cloud: gardenv1beta1.SeedCloud{
						Profile: cloudProfileName,
						Region:  regionEuropeWest,
					},
				},
			}

			zones = []gardenv1beta1.Zone{
				{
					Region: regionEuropeWest,
				},
				{
					Region: regionEuropeEast,
					Names: []string{
						"europe-east1-b",
						"europe-east1-c",
					},
				},
			}

			cloudProfileGCEBase = gardenv1beta1.GCPProfile{
				Constraints: gardenv1beta1.GCPConstraints{
					Zones: zones,
				},
			}

			cloudProfileBase = gardenv1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: cloudProfileName,
				},
				Spec: gardenv1beta1.CloudProfileSpec{
					GCP: &cloudProfileGCEBase,
				},
			}
		)

		BeforeEach(func() {
			cloudProfile = *cloudProfileBase.DeepCopy()
			seed = *seedBase.DeepCopy()
			allSeeds = []gardenv1beta1.Seed{
				seed,
			}
		})

		It("GCP", func() {
			unsupportedRegion, zones, err := ChooseRegionAndZoneWithNoSeed(gardenv1beta1.CloudProviderGCP, zones, &cloudProfile, allSeeds)
			Expect(err).ToNot(HaveOccurred())
			Expect(*unsupportedRegion).To(Equal(regionEuropeEast))
			Expect(zones).ToNot(Equal(nil))
			Expect(len(zones)).ToNot(Equal(0))
		})
		It("AWS", func() {
			unsupportedRegion, zones, err := ChooseRegionAndZoneWithNoSeed(gardenv1beta1.CloudProviderAWS, zones, &cloudProfile, allSeeds)
			Expect(err).ToNot(HaveOccurred())
			Expect(unsupportedRegion).ToNot(Equal(nil))
			Expect(*unsupportedRegion).To(Equal(regionEuropeEast))
			Expect(zones).ToNot(Equal(nil))
			Expect(len(zones)).ToNot(Equal(0))
		})
		It("Alicloud", func() {
			unsupportedRegion, zones, err := ChooseRegionAndZoneWithNoSeed(gardenv1beta1.CloudProviderAlicloud, zones, &cloudProfile, allSeeds)
			Expect(err).ToNot(HaveOccurred())
			Expect(unsupportedRegion).ToNot(Equal(nil))
			Expect(*unsupportedRegion).To(Equal(regionEuropeEast))
			Expect(zones).ToNot(Equal(nil))
			Expect(len(zones)).ToNot(Equal(0))
		})
		It("Openstack", func() {
			unsupportedRegion, zones, err := ChooseRegionAndZoneWithNoSeed(gardenv1beta1.CloudProviderOpenStack, zones, &cloudProfile, allSeeds)
			Expect(err).ToNot(HaveOccurred())
			Expect(unsupportedRegion).ToNot(Equal(nil))
			Expect(*unsupportedRegion).To(Equal(regionEuropeEast))
			Expect(zones).ToNot(Equal(nil))
			Expect(len(zones)).ToNot(Equal(0))
		})
		It("Packet", func() {
			unsupportedRegion, zones, err := ChooseRegionAndZoneWithNoSeed(gardenv1beta1.CloudProviderPacket, zones, &cloudProfile, allSeeds)
			Expect(err).ToNot(HaveOccurred())
			Expect(unsupportedRegion).ToNot(Equal(nil))
			Expect(*unsupportedRegion).To(Equal(regionEuropeEast))
			Expect(zones).ToNot(Equal(nil))
			Expect(len(zones)).ToNot(Equal(0))
		})
		It("Azure", func() {

			azureRegionWestEurope := "westeurope"
			azureRegionEastEurope := "eastus"
			var (
				westeurope = gardenv1beta1.AzureDomainCount{
					Region: azureRegionWestEurope,
					Count:  5,
				}
				eastEurope = gardenv1beta1.AzureDomainCount{
					Region: azureRegionEastEurope,
					Count:  5,
				}
				failureDomainCounts = []gardenv1beta1.AzureDomainCount{
					westeurope,
					eastEurope,
				}
				updateDomainCounts = []gardenv1beta1.AzureDomainCount{
					westeurope,
					eastEurope,
				}
				cloudProfileAzureBase = gardenv1beta1.AzureProfile{
					CountUpdateDomains: updateDomainCounts,
					CountFaultDomains:  failureDomainCounts,
				}
				cloudProfileSpecAzure = gardenv1beta1.CloudProfileSpec{
					Azure: &cloudProfileAzureBase,
				}
			)

			seed.Spec.Cloud.Region = azureRegionWestEurope
			allSeedsAzure := []gardenv1beta1.Seed{
				seed,
			}

			cloudProfile.Spec = cloudProfileSpecAzure
			unsupportedRegion, _, err := ChooseRegionAndZoneWithNoSeed(gardenv1beta1.CloudProviderAzure, []gardenv1beta1.Zone{}, &cloudProfile, allSeedsAzure)
			Expect(err).ToNot(HaveOccurred())
			Expect(unsupportedRegion).ToNot(Equal(nil))
			Expect(*unsupportedRegion).To(Equal(azureRegionEastEurope))
		})
	})
})
