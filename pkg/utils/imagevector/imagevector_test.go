// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package imagevector_test

import (
	"fmt"

	. "github.com/gardener/gardener/pkg/utils/imagevector"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("imagevector", func() {
	Describe("> ImageVector", func() {
		var vector ImageVector

		BeforeEach(func() {
			vector = ImageVector{}
		})

		Describe("#FindImage", func() {
			var (
				k8s164 = "1.6.4"
				k8s180 = "1.8.0"

				imageSrc1 = &ImageSource{
					Name:       "image1",
					Repository: "repo1",
					Tag:        "tag1",
					Versions:   "",
				}
				imageSrc2 = &ImageSource{
					Name:       "image1",
					Repository: "repo1",
					Tag:        "tag1",
					Versions:   ">= 1.6",
				}
				imageSrc3 = &ImageSource{
					Name:       "image3",
					Repository: "repo3",
					Tag:        "tag3",
					Versions:   ">= 1.6, < 1.8",
				}
				imageSrc4 = &ImageSource{
					Name:       "image3",
					Repository: "repo3",
					Tag:        "tag3",
					Versions:   ">= 1.8",
				}
				imageSrc5 = &ImageSource{
					Name:       "image5",
					Repository: "repo5",
				}
			)

			It("should return an error because no image was found", func() {
				image, err := vector.FindImage("test", k8s164, k8s164)

				Expect(err).To(HaveOccurred())
				Expect(image).To(BeNil())
			})

			It("should return an image because it only exists once in the vector", func() {
				vector = ImageVector{imageSrc1}

				image, err := vector.FindImage(imageSrc1.Name, k8s164, k8s180)

				Expect(err).NotTo(HaveOccurred())
				Expect(image).To(Equal(imageSrc1.ToImage(k8s180)))
			})

			It("should return an image which exists multiple times after it has checked the constraints (first image returned)", func() {
				vector = ImageVector{imageSrc3, imageSrc4}

				image, err := vector.FindImage(imageSrc3.Name, k8s164, k8s164)

				Expect(err).NotTo(HaveOccurred())
				Expect(image).To(Equal(imageSrc3.ToImage(k8s164)))
			})

			It("should return an image which exists multiple times after it has checked the constraints (second image returned)", func() {
				vector = ImageVector{imageSrc3, imageSrc4}

				image, err := vector.FindImage(imageSrc3.Name, k8s180, k8s180)

				Expect(err).NotTo(HaveOccurred())
				Expect(image).To(Equal(imageSrc4.ToImage(k8s180)))
			})

			It("should return an error for an image which exists multiple times after it has checked the constraints (no constraints met)", func() {
				vector = ImageVector{imageSrc3, imageSrc4}

				image, err := vector.FindImage(imageSrc3.Name, "1.5.9", "1.5.9")

				Expect(err).To(HaveOccurred())
				Expect(image).To(BeNil())
			})

			It("should return an image which exists multiple times (no version constraints provided)", func() {
				vector = ImageVector{imageSrc1, imageSrc2}

				image, err := vector.FindImage(imageSrc1.Name, k8s164, k8s164)

				Expect(err).NotTo(HaveOccurred())
				Expect(image).To(Equal(imageSrc1.ToImage(k8s164)))
			})

			It("should return an image where the version was correctly applied", func() {
				vector = ImageVector{imageSrc5}

				image, err := vector.FindImage(imageSrc5.Name, k8s164, k8s164)

				Expect(err).NotTo(HaveOccurred())
				Expect(image).To(Equal(imageSrc5.ToImage(k8s164)))
			})
		})

		Describe("#FindImages", func() {
			var (
				k8s164 = "1.6.4"
				k8s180 = "1.8.0"

				imageSrc1 = &ImageSource{
					Name:       "image1",
					Repository: "repo1",
					Tag:        "tag1",
					Versions:   "",
				}
				imageSrc2 = &ImageSource{
					Name:       "image2",
					Repository: "repo2",
					Tag:        "tag2",
					Versions:   "",
				}
			)

			It("should return an error because one or more images was not found", func() {
				images, err := vector.FindImages([]string{"test"}, k8s164, k8s164)

				Expect(err).To(HaveOccurred())
				Expect(images).To(BeNil())
			})

			It("should return an image because it only exists once in the vector", func() {
				vector = ImageVector{imageSrc1, imageSrc2}
				expectMap := map[string]interface{}{
					"image1": imageSrc1.ToImage("").String(),
					"image2": imageSrc2.ToImage("").String(),
				}

				images, err := vector.FindImages([]string{imageSrc1.Name, imageSrc2.Name}, k8s164, k8s180)

				Expect(err).NotTo(HaveOccurred())
				Expect(images).To(Equal(expectMap))
			})
		})
	})

	Describe("> Image", func() {
		Describe("#String", func() {
			It("should return the string representation of the image (w/o tag)", func() {
				var (
					repo = "my-repo"
					tag  = "1.2.3"
				)

				image := Image{
					Name:       "my-image",
					Repository: repo,
					Tag:        tag,
				}

				Expect(image.String()).To(Equal(fmt.Sprintf("%s:%s", repo, tag)))
			})

			It("should return the string representation of the image (w/ tag)", func() {
				repo := "my-repo"

				image := Image{
					Name:       "my-image",
					Repository: repo,
				}

				Expect(image.String()).To(Equal(repo))
			})
		})
	})

	Describe("> ImageSource", func() {
		Describe("#ToImage", func() {
			It("should return an image with the same tag", func() {
				var (
					name       = "foo"
					repository = "repo"
					tag        = "v1"

					source = ImageSource{
						Name:       name,
						Repository: repository,
						Tag:        tag,
					}
				)

				image := source.ToImage("1.8.0")

				Expect(image).To(Equal(&Image{
					Name:       name,
					Repository: repository,
					Tag:        tag,
				}))
			})

			It("should return an image with the given version as tag", func() {
				var (
					name       = "foo"
					repository = "repo"
					version    = "1.8.0"

					source = ImageSource{
						Name:       name,
						Repository: repository,
					}
				)

				image := source.ToImage(version)

				Expect(image).To(Equal(&Image{
					Name:       name,
					Repository: repository,
					Tag:        fmt.Sprintf("v%s", version),
				}))
			})
		})
	})
})
