// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import (
	"archive/tar"
	"bytes"
	"io/fs"

	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/fake"
	"github.com/google/go-containerregistry/pkg/v1/static"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
)

var _ = Describe("#exctractFromDownloadedImage", func() {
	var (
		extractor remoteExtractor
		image     *fake.FakeImage
	)

	BeforeEach(func() {
		extractor = remoteExtractor{
			fs: afero.NewMemMapFs(),
		}

		image = &fake.FakeImage{}
	})

	Context("with single layer fs", func() {
		var imageFs afero.Fs

		BeforeEach(func() {
			imageFs = afero.NewMemMapFs()
			image.LayersStub = func() ([]containerregistryv1.Layer, error) {
				return fsToTarLayer(imageFs)
			}
		})

		It("extracts gardener-node-agent from image to destination", func() {
			Expect(afero.WriteFile(imageFs, "/bin/gardener-node-agent", []byte("[binary data]"), fs.ModePerm)).Should(Succeed())

			Expect(
				extractor.extractFromDownloadedImage(image, "gardener-node-agent", "/out/gardener-node-agent"),
			).Should(Succeed())

			data, err := afero.ReadFile(extractor.fs, "/out/gardener-node-agent")
			Expect(err).ShouldNot(HaveOccurred())

			Expect(string(data)).To(Equal("[binary data]"))
		})

		It("extracts specific config from image to destination", func() {
			Expect(afero.WriteFile(imageFs, "/etc/correct.config", []byte("{\"expected\":true}"), fs.ModePerm)).Should(Succeed())
			Expect(afero.WriteFile(imageFs, "/etc/wrong.config", []byte("{\"expected\":false}"), fs.ModePerm)).Should(Succeed())

			Expect(
				extractor.extractFromDownloadedImage(image, "correct.config", "/out/got.config"),
			).Should(Succeed())

			data, err := afero.ReadFile(extractor.fs, "/out/got.config")
			Expect(err).ShouldNot(HaveOccurred())

			Expect(string(data)).To(Equal("{\"expected\":true}"))
		})

		It("fails to extract on miss", func() {
			Expect(
				extractor.extractFromDownloadedImage(image, "missing.config", "/out/got.config"),
			).Should(MatchError(ContainSubstring("could not find file %q in layer", "missing.config")))

			_, err := afero.ReadFile(extractor.fs, "/out/got.config")
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("with multi layer fs", func() {
		var fsLayers []afero.Fs

		BeforeEach(func() {
			fsLayers = make([]afero.Fs, 0)
			image.LayersStub = func() ([]containerregistryv1.Layer, error) {
				layers := make([]containerregistryv1.Layer, 0, len(fsLayers))
				for _, layerFs := range fsLayers {
					ls, err := fsToTarLayer(layerFs)
					if err != nil {
						return nil, err
					}
					layers = append(layers, ls...)
				}
				return layers, nil
			}
		})

		It("ignores empty layers", func() {
			baseLayer := afero.NewMemMapFs()
			Expect(afero.WriteFile(baseLayer, "/bin/gardener-node-agent", []byte("[binary data]"), fs.ModePerm)).Should(Succeed())
			emptyLayer := afero.NewMemMapFs()
			fsLayers = append(fsLayers, baseLayer, emptyLayer)

			Expect(
				extractor.extractFromDownloadedImage(image, "gardener-node-agent", "/out/gardener-node-agent"),
			).Should(Succeed())

			data, err := afero.ReadFile(extractor.fs, "/out/gardener-node-agent")
			Expect(err).ShouldNot(HaveOccurred())

			Expect(string(data)).To(Equal("[binary data]"))
		})

		It("top layers override base layer", func() {
			baseLayer := afero.NewMemMapFs()
			Expect(afero.WriteFile(baseLayer, "/bin/gardener-node-agent", []byte("[base data]"), fs.ModePerm)).Should(Succeed())
			topLayer := afero.NewMemMapFs()
			Expect(afero.WriteFile(baseLayer, "/bin/gardener-node-agent", []byte("[top data]"), fs.ModePerm)).Should(Succeed())
			fsLayers = append(fsLayers, baseLayer, topLayer)

			Expect(
				extractor.extractFromDownloadedImage(image, "gardener-node-agent", "/out/gardener-node-agent"),
			).Should(Succeed())

			data, err := afero.ReadFile(extractor.fs, "/out/gardener-node-agent")
			Expect(err).ShouldNot(HaveOccurred())

			Expect(string(data)).To(Equal("[top data]"))
		})
	})
})

func fsToTarLayer(layerFs afero.Fs) ([]containerregistryv1.Layer, error) {
	var layerBuffer bytes.Buffer
	tarWriter := tar.NewWriter(&layerBuffer)

	err := afero.Walk(layerFs, "/", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return err
		}
		err = tarWriter.WriteHeader(header)
		if err != nil {
			return err
		}
		data, err := afero.ReadFile(layerFs, path)
		if err != nil {
			return nil
		}
		_, err = tarWriter.Write(data)
		return err
	})
	if err != nil {
		return nil, err
	}

	return []containerregistryv1.Layer{
		static.NewLayer(layerBuffer.Bytes(), "tar"),
	}, nil
}
