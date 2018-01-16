// Copyright 2018 The Gardener Authors.
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
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("imagevector", func() {
	Describe("> ImageVector", func() {
		var vector ImageVector

		BeforeEach(func() {
			vector = ImageVector{}
		})

		Describe("#FindImage", func() {
			var (
				image1 = &Image{
					Name:       "image1",
					Repository: "repo1",
					Tag:        "tag1",
					Versions:   "",
				}
				image2 = &Image{
					Name:       "image1",
					Repository: "repo1",
					Tag:        "tag1",
					Versions:   ">= 1.6",
				}
				image3 = &Image{
					Name:       "image3",
					Repository: "repo3",
					Tag:        "tag3",
					Versions:   ">= 1.6, < 1.8",
				}
				image4 = &Image{
					Name:       "image3",
					Repository: "repo3",
					Tag:        "tag3",
					Versions:   ">= 1.8",
				}
			)

			It("should return an error because no image was found", func() {
				image, err := vector.FindImage("test", "1.6.4")

				Expect(err).To(HaveOccurred())
				Expect(image).To(BeNil())
			})

			It("should return an image because it only exists once in the vector", func() {
				vector = ImageVector{image1}

				image, err := vector.FindImage(image1.Name, "1.6.4")

				Expect(err).NotTo(HaveOccurred())
				Expect(*image).To(MatchFields(IgnoreExtras, Fields{
					"Name":       Equal(image1.Name),
					"Repository": Equal(image1.Repository),
					"Tag":        Equal(image1.Tag),
					"Versions":   Equal(image1.Versions),
				}))
			})

			It("should return an image which exists multiple times after it has checked the constraints (first image returned)", func() {
				vector = ImageVector{image3, image4}

				image, err := vector.FindImage(image3.Name, "1.6.4")

				Expect(err).NotTo(HaveOccurred())
				Expect(*image).To(MatchFields(IgnoreExtras, Fields{
					"Name":       Equal(image3.Name),
					"Repository": Equal(image3.Repository),
					"Tag":        Equal(image3.Tag),
					"Versions":   Equal(image3.Versions),
				}))
			})

			It("should return an image which exists multiple times after it has checked the constraints (second image returned)", func() {
				vector = ImageVector{image3, image4}

				image, err := vector.FindImage(image3.Name, "1.8.0")

				Expect(err).NotTo(HaveOccurred())
				Expect(*image).To(MatchFields(IgnoreExtras, Fields{
					"Name":       Equal(image4.Name),
					"Repository": Equal(image4.Repository),
					"Tag":        Equal(image4.Tag),
					"Versions":   Equal(image4.Versions),
				}))
			})

			It("should return an error for an image which exists multiple times after it has checked the constraints (no constraints met)", func() {
				vector = ImageVector{image3, image4}

				image, err := vector.FindImage(image3.Name, "1.5.9")

				Expect(err).To(HaveOccurred())
				Expect(image).To(BeNil())
			})

			It("should return an image which exists multiple times (no version constraints provided)", func() {
				vector = ImageVector{image1, image2}

				image, err := vector.FindImage(image1.Name, "1.6.4")

				Expect(err).NotTo(HaveOccurred())
				Expect(*image).To(MatchFields(IgnoreExtras, Fields{
					"Name":       Equal(image1.Name),
					"Repository": Equal(image1.Repository),
					"Tag":        Equal(image1.Tag),
					"Versions":   Equal(image1.Versions),
				}))
			})
		})
	})

	Describe("> Image", func() {
		var image Image

		BeforeEach(func() {
			image = Image{}
		})

		Describe("#String", func() {
			It("should return the string representation of the image (w/o tag)", func() {
				var (
					repo = "my-repo"
					tag  = "1.2.3"
				)

				image = Image{
					Name:       "my-image",
					Repository: repo,
					Tag:        tag,
				}

				Expect(image.String()).To(Equal(fmt.Sprintf("%s:%s", repo, tag)))
			})

			It("should return the string representation of the image (w/ tag)", func() {
				repo := "my-repo"

				image = Image{
					Name:       "my-image",
					Repository: repo,
				}

				Expect(image.String()).To(Equal(repo))
			})
		})
	})
})
