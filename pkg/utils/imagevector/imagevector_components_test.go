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
