// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package imagevector_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/imagevector"
)

var _ = Describe("imagevector", func() {
	Describe("#ComponentImageVectors", func() {
		var (
			component1     = "foo"
			componentData1 = "images: []"

			component2     = "bar"
			componentData2 = "images: []"

			componentImageVectors = ComponentImageVectors{
				component1: componentData1,
				component2: componentData2,
			}

			componentImagesJSON = fmt.Sprintf(`
{
	"components": [
		{
			"name": "%s",
			"imageVectorOverwrite": "%s"
		},
		{
			"name": "%s",
			"imageVectorOverwrite": "%s"
		},
	]
}`, component1, componentData1, component2, componentData2)

			componentImagesYAML = fmt.Sprintf(`
components:
- name: %s
  imageVectorOverwrite: "%s"
- name: %s
  imageVectorOverwrite: "%s"
`, component1, componentData1, component2, componentData2)
		)

		Describe("#ReadComponentOverwrite", func() {
			It("should successfully read a JSON image vector", func() {
				vector, err := ReadComponentOverwrite([]byte(componentImagesJSON))
				Expect(err).NotTo(HaveOccurred())
				Expect(vector).To(Equal(componentImageVectors))
			})

			It("should successfully read a YAML image vector", func() {
				vector, err := ReadComponentOverwrite([]byte(componentImagesYAML))
				Expect(err).NotTo(HaveOccurred())
				Expect(vector).To(Equal(componentImageVectors))
			})
		})

		Describe("#ReadComponentOverwriteFile", func() {
			It("should successfully read the file and close it", func() {
				tmpFile, cleanup := withTempFile("component imagevector", []byte(componentImagesJSON))
				defer cleanup()

				vector, err := ReadComponentOverwriteFile(tmpFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(vector).To(Equal(componentImageVectors))
			})
		})
	})
})
