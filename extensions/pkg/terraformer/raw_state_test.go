// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terraformer_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener/extensions/pkg/terraformer"
)

const (
	testJsonArray = `{"test": 1}`
)

var _ = Describe("raw_state", func() {

	Describe("#UnmarshalRawState", func() {
		It("should unmarshal successfully json string and have NoneEncoding", func() {
			rs, err := terraformer.UnmarshalRawState(testJsonArray)
			Expect(err).ToNot(HaveOccurred())
			Expect(rs.Encoding).To(Equal(terraformer.NoneEncoding))
		})

		It("should unmarshal successfully nill and have NoneEncoding", func() {
			rs, err := terraformer.UnmarshalRawState(nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(rs.Encoding).To(Equal(terraformer.NoneEncoding))
		})
		It("should unmarshal successfully []byte and have NoneEncoding", func() {
			rs, err := terraformer.UnmarshalRawState([]byte(testJsonArray))
			Expect(err).ToNot(HaveOccurred())
			Expect(rs.Encoding).To(Equal(terraformer.NoneEncoding))
		})
		It("should unmarshal successfully RawExtension and have NoneEncoding", func() {
			re := &runtime.RawExtension{
				Raw: []byte(testJsonArray),
			}
			rs, err := terraformer.UnmarshalRawState(re)
			Expect(err).ToNot(HaveOccurred())
			Expect(rs.Encoding).To(Equal(terraformer.NoneEncoding))
		})
		It("should not unmarshal successfully RawExtension because of invalid data type", func() {
			_, err := terraformer.UnmarshalRawState(1)
			Expect(err).To(HaveOccurred())
		})
		It("should not unmarshal successfully RawExtension because of invalid data", func() {
			_, err := terraformer.UnmarshalRawState("NOT JSON")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#MarshalRawState", func() {
		It("should marshal and then unmarshall successfully RawExtension", func() {
			re := &terraformer.RawState{
				Data: testJsonArray,
			}
			data, err := re.Marshal()
			Expect(err).ToNot(HaveOccurred())

			rs, err := terraformer.UnmarshalRawState(data)
			Expect(err).ToNot(HaveOccurred())
			Expect(rs.Encoding).To(Equal(terraformer.NoneEncoding))
			Expect(rs.Data).To(Equal(testJsonArray))
		})
	})
})
