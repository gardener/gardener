// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package chart

import (
	"testing"

	"github.com/gardener/gardener/pkg/utils/imagevector"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestChart(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Chart Suite")
}

var _ = Describe("Chart", func() {
	Describe("ImageMapToValues", func() {
		It("should transform the given image map to values", func() {
			var (
				img1 = &imagevector.Image{
					Name:       "img1",
					Repository: "repo1",
				}
				img2 = &imagevector.Image{
					Name:       "img2",
					Repository: "repo2",
				}
			)

			values := ImageMapToValues(map[string]*imagevector.Image{
				img1.Name: img1,
				img2.Name: img2,
			})
			Expect(values).To(Equal(map[string]interface{}{
				img1.Name: img1.String(),
				img2.Name: img2.String(),
			}))
		})
	})

	Describe("#InjectImages", func() {
		It("should find the images and inject the image as value map at the 'images' key into a shallow copy", func() {
			var (
				values map[string]interface{}
				img1   = &imagevector.ImageSource{
					Name:       "img1",
					Repository: "repo1",
				}
				img2 = &imagevector.ImageSource{
					Name:       "img2",
					Repository: "repo2",
				}
				v = imagevector.ImageVector{img1, img2}
			)

			injected, err := InjectImages(values, v, []string{img1.Name, img2.Name})
			Expect(err).NotTo(HaveOccurred())
			Expect(injected).To(Equal(map[string]interface{}{
				"images": map[string]interface{}{
					img1.Name: img1.ToImage(nil).String(),
					img2.Name: img2.ToImage(nil).String(),
				},
			}))
		})
	})

	Describe("#CopyValues", func() {
		It("should create a shallow copy of the map", func() {
			v := map[string]interface{}{"foo": nil, "bar": map[string]interface{}{"baz": nil}}

			c := CopyValues(v)

			Expect(c).To(Equal(v))

			c["foo"] = 1
			Expect(v["foo"]).To(BeNil())

			c["bar"].(map[string]interface{})["baz"] = "bang"
			Expect(v["bar"].(map[string]interface{})["baz"]).To(Equal("bang"))
		})
	})
})
