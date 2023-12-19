// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package chart

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/utils/imagevector"
)

func TestChart(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Chart Suite")
}

var _ = Describe("Chart", func() {
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
})
