// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
				Logger:       logger.AddWriter(logger.NewLogger("info"), GinkgoWriter),
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
