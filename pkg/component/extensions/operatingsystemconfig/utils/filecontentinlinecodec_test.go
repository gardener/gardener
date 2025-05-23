// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/utils"
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
