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

	"github.com/gardener/gardener/test/framework"

	"github.com/gardener/gardener/pkg/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestFramework(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Framework Test Suite")
}

var _ = Describe("Framework tests", func() {

	Context("Download Chart Artifacts", func() {

		var f *framework.CommonFramework

		AfterEach(func() {
			err := os.RemoveAll(f.ChartDir)
			Expect(err).NotTo(HaveOccurred())

			err = os.RemoveAll(filepath.Join(f.ResourcesDir, "repository", "cache"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should download chart artifacts", func() {

			f = &framework.CommonFramework{
				Logger:       logger.AddWriter(logger.NewLogger("info", ""), GinkgoWriter),
				ResourcesDir: "./resources",
				ChartDir:     "./resources/charts",
			}

			helmRepo := framework.Helm(f.ResourcesDir)
			err := framework.EnsureRepositoryDirectories(helmRepo)
			Expect(err).NotTo(HaveOccurred())

			err = f.DownloadChartArtifacts(context.TODO(), helmRepo, f.ChartDir, "stable/redis", "7.0.0")
			Expect(err).NotTo(HaveOccurred())

			expectedCachePath := filepath.Join(f.ResourcesDir, "repository", "cache", "stable-index.yaml")
			cacheIndexExists, err := framework.Exists(expectedCachePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(cacheIndexExists).To(BeTrue())

			expectedRedisChartPath := filepath.Join(f.ResourcesDir, "charts", "redis")
			chartExists, err := framework.Exists(expectedRedisChartPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(chartExists).To(BeTrue())
		})
	})

})
