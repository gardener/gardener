// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package chartrenderer_test

import (
	"embed"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/version"

	"github.com/gardener/gardener/pkg/chartrenderer"
)

const alpinePod = `apiVersion: v1
kind: Pod
metadata:
  name: alpine
  namespace: default
  labels:
    chartName: alpine
    chartVersion: "0.1.0"
spec:
  restartPolicy: Never
  containers:
  - name: waiter
    image: alpine:3.3
    command: ["/bin/sleep", "9000"]
`

//go:embed testdata/alpine/*
var embeddedFS embed.FS

var _ = Describe("ChartRenderer", func() {
	var (
		alpineChartPath = filepath.Join("testdata", "alpine")
		renderer        chartrenderer.Interface
	)

	BeforeEach(func() {
		renderer = chartrenderer.NewWithServerVersion(&version.Info{})
	})

	Describe("#RenderEmbeddedFS", func() {
		It("should return err when chartPath is missing", func() {
			_, err := renderer.RenderEmbeddedFS(embeddedFS, filepath.Join("testdata", "missing"), "missing", "default", map[string]string{})
			Expect(err).To(MatchError(ContainSubstring(`can't load chart "testdata/missing" from embedded file system`)))
		})

		It("should return rendered chart", func() {
			chart, err := renderer.RenderEmbeddedFS(embeddedFS, alpineChartPath, "alpine", "default", map[string]string{})
			Expect(err).ToNot(HaveOccurred())

			files := chart.Files()
			Expect(files).To(HaveLen(2))
			Expect(files).To(HaveKeyWithValue("alpine/templates/alpine-pod.yaml", alpinePod))
		})
	})

	Describe("#FileContent", func() {
		It("should return empty string when template file is missing", func() {
			chart, err := renderer.RenderEmbeddedFS(embeddedFS, alpineChartPath, "alpine", "default", map[string]string{})
			Expect(err).ToNot(HaveOccurred())

			actual := chart.FileContent("missing.yaml")
			Expect(actual).To(BeEmpty())
		})

		It("should return the file content when template file exists", func() {
			chart, err := renderer.RenderEmbeddedFS(embeddedFS, alpineChartPath, "alpine", "default", map[string]string{})
			Expect(err).ToNot(HaveOccurred())

			actual := chart.FileContent("alpine-pod.yaml")
			Expect(actual).To(Equal(alpinePod))
		})
	})

	Describe("#Manifest", func() {
		It("should return manifest", func() {
			chart, err := renderer.RenderEmbeddedFS(embeddedFS, alpineChartPath, "alpine", "default", map[string]string{})
			Expect(err).ToNot(HaveOccurred())

			manifests := chart.Manifest()
			Expect(manifests).NotTo(BeNil())
		})
	})

	Describe("#AsSecretData", func() {
		It("should return rendered chart as secret data", func() {
			chart, err := renderer.RenderEmbeddedFS(embeddedFS, alpineChartPath, "alpine", "default", map[string]string{})
			Expect(err).ToNot(HaveOccurred())

			data := chart.AsSecretData()
			Expect(data).To(Not(BeNil()))
			Expect(string(data["alpine_templates_alpine-pod.yaml"])).To(Equal(alpinePod))
		})
	})
})
