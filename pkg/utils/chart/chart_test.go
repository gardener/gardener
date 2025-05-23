// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package chart_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/utils/chart"
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
				values map[string]any
				img1   = &imagevector.ImageSource{
					Name:       "img1",
					Repository: ptr.To("repo1"),
				}
				img2 = &imagevector.ImageSource{
					Name:       "img2",
					Repository: ptr.To("repo2"),
				}
				v = imagevector.ImageVector{img1, img2}
			)

			injected, err := InjectImages(values, v, []string{img1.Name, img2.Name})
			Expect(err).NotTo(HaveOccurred())
			Expect(injected).To(Equal(map[string]any{
				"images": map[string]any{
					img1.Name: img1.ToImage(nil).String(),
					img2.Name: img2.ToImage(nil).String(),
				},
			}))
		})
	})
})
