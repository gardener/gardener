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

package framework

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
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
			seed             gardencorev1alpha1.Seed
			allSeeds         []gardencorev1alpha1.Seed
			seedName         = "seed-1"
			regionEuropeWest = "europe-west1"
			regionEuropeEast = "us-east1"
			providerType     = "aws"

			seedBase = gardencorev1alpha1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
				Spec: gardencorev1alpha1.SeedSpec{
					Provider: gardencorev1alpha1.SeedProvider{Type: providerType, Region: regionEuropeWest},
				},
			}

			regions = []gardencorev1alpha1.Region{
				{
					Name: regionEuropeWest,
				},
				{
					Name: regionEuropeEast,
					Zones: []gardencorev1alpha1.AvailabilityZone{
						{
							Name: "europe-east1-b",
						},
						{
							Name: "europe-east1-c",
						},
					},
				},
			}
		)

		BeforeEach(func() {
			seed = *seedBase.DeepCopy()
			allSeeds = []gardencorev1alpha1.Seed{
				seed,
			}
		})

		It("Unsupported Provider is being returned", func() {
			unsupportedRegion, err := ChooseRegionAndZoneWithNoSeed(regions, allSeeds)
			Expect(err).ToNot(HaveOccurred())
			Expect(unsupportedRegion.Name).To(Equal(regionEuropeEast))
			Expect(len(unsupportedRegion.Zones)).ToNot(Equal(0))
		})
	})
})
