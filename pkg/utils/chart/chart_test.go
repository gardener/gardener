// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
