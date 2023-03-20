// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
		It("shoud unmarshal successfully json string and have NoneEncoding", func() {
			rs, err := terraformer.UnmarshalRawState(testJsonArray)
			Expect(err).ToNot(HaveOccurred())
			Expect(rs.Encoding).To(Equal(terraformer.NoneEncoding))
		})

		It("shoud unmarshal successfully nill and have NoneEncoding", func() {
			rs, err := terraformer.UnmarshalRawState(nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(rs.Encoding).To(Equal(terraformer.NoneEncoding))
		})
		It("shoud unmarshal successfully []byte and have NoneEncoding", func() {
			rs, err := terraformer.UnmarshalRawState([]byte(testJsonArray))
			Expect(err).ToNot(HaveOccurred())
			Expect(rs.Encoding).To(Equal(terraformer.NoneEncoding))
		})
		It("shoud unmarshal successfully RawExtension and have NoneEncoding", func() {
			re := &runtime.RawExtension{
				Raw: []byte(testJsonArray),
			}
			rs, err := terraformer.UnmarshalRawState(re)
			Expect(err).ToNot(HaveOccurred())
			Expect(rs.Encoding).To(Equal(terraformer.NoneEncoding))
		})
		It("shoud not unmarshal successfully RawExtension because of invalid data type", func() {
			_, err := terraformer.UnmarshalRawState(1)
			Expect(err).To(HaveOccurred())
		})
		It("shoud not unmarshal successfully RawExtension because of invalid data", func() {
			_, err := terraformer.UnmarshalRawState("NOT JSON")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#MarshalRawState", func() {
		It("shoud marshal and then unmarshall successfully RawExtension", func() {
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
