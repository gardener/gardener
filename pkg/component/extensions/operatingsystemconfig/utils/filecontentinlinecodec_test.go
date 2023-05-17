// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package utils

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var _ = Describe("FileContentInlineCodec", func() {
	var (
		data = []byte(`apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
`)

		fileContent = &extensionsv1alpha1.FileContentInline{
			Encoding: "b64",
			Data:     `YXBpVmVyc2lvbjoga3ViZWxldC5jb25maWcuazhzLmlvL3YxYmV0YTEKa2luZDogS3ViZWxldENvbmZpZ3VyYXRpb24K`,
		}
	)

	Describe("#Encode", func() {
		It("should encode the given byte slice into a FileContentInline appropriately", func() {
			// Create codec
			c := NewFileContentInlineCodec()

			// Call Encode and check result
			fci, err := c.Encode(data, "b64")
			Expect(err).NotTo(HaveOccurred())
			Expect(fci).To(Equal(fileContent))
		})
	})

	Describe("#Decode", func() {
		It("should decode a byte slice from the given FileContentInline appropriately", func() {
			// Create codec
			c := NewFileContentInlineCodec()

			// Call Decode and check result
			d, err := c.Decode(fileContent)
			Expect(err).NotTo(HaveOccurred())
			Expect(d).To(Equal(data))
		})
	})
})
