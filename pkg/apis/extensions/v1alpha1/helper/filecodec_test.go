// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
)

var _ = Describe("filecodec", func() {
	DescribeTable("#EncodeDecode",
		func(input extensionsv1alpha1.FileContentInline) {
			codeID, err := ParseFileCodecID(input.Encoding)
			Expect(err).NotTo(HaveOccurred())
			encoded, err := FileCodecForID(codeID).Encode([]byte(input.Data))
			Expect(err).NotTo(HaveOccurred())

			decoded, err := Decode(input.Encoding, encoded)
			Expect(err).NotTo(HaveOccurred())
			Expect(input.Data).To(Equal(string(decoded)))
		},

		Entry("plain", extensionsv1alpha1.FileContentInline{Encoding: "", Data: "plain data input"}),
		Entry("base64", extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: "base64 data input"}),
	)
})
