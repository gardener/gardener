// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/cloudinit"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FileContentInlineCodec", func() {
	var (
		data = []byte(`apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
`)

		fileContent = &extensionsv1alpha1.FileContentInline{
			Encoding: string(cloudinit.B64FileCodecID),
			Data:     `YXBpVmVyc2lvbjoga3ViZWxldC5jb25maWcuazhzLmlvL3YxYmV0YTEKa2luZDogS3ViZWxldENvbmZpZ3VyYXRpb24K`,
		}
	)

	Describe("#Encode", func() {
		It("should encode the given byte slice into a FileContentInline appropriately", func() {
			// Create codec
			c := NewFileContentInlineCodec()

			// Call Encode and check result
			fci, err := c.Encode(data, string(cloudinit.B64FileCodecID))
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
