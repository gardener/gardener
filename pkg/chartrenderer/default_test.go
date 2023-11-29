// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package chartrenderer_test

import (
	"embed"
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/version"

	"github.com/gardener/gardener/pkg/chartrenderer"
)

const (
	alpinePod = `apiVersion: v1
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
    command: ["/bin/sleep", "9000"]`

	testSecret = `apiVersion: v1
kind: Secret
metadata:
  name: test
  namespace: default
  labels:
    chartName: alpine
    chartVersion: "0.1.0"
type: Opaque`
)

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
			Expect(files).To(HaveKey("alpine/templates/alpine-pod.yaml"))
			Expect(files["alpine/templates/alpine-pod.yaml"]).To(HaveKeyWithValue("pod/alpine", alpinePod))
			Expect(files["alpine/templates/alpine-pod.yaml"]).To(HaveKeyWithValue("secret/test", testSecret))
			Expect(files).To(HaveKey("alpine/templates/secret.yaml"))
			Expect(files["alpine/templates/secret.yaml"]).To(HaveKeyWithValue("secret/test", testSecret))
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
			Expect(actual).To(Equal(testSecret + "\n---\n" + alpinePod))

			actual = chart.FileContent("secret.yaml")
			Expect(actual).To(Equal(testSecret))
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
			fmt.Print(data)
			Expect(string(data["alpine_templates_secret.yaml"])).To(Equal(testSecret))
			Expect(string(data["alpine_templates_alpine-pod_secret_test.yaml"])).To(Equal(testSecret))
			Expect(string(data["alpine_templates_alpine-pod_pod_alpine.yaml"])).To(Equal(alpinePod))
		})
	})
})
