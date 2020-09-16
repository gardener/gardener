// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	. "github.com/gardener/gardener/pkg/utils/secrets"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StaticToken Data", func() {
	var (
		staticTokenJSON     = []byte(`{"tokens":{"foo":"foo"}}`)
		staticTokenInfoData = &StaticTokenInfoData{
			Tokens: map[string]string{
				"foo": "foo",
			},
		}
	)

	Describe("#UnmarshalStaticToken", func() {
		It("should properly unmarshal StaticTokenJSONData into StaticTokenInfoData", func() {
			infoData, err := UnmarshalStaticToken(staticTokenJSON)
			Expect(err).NotTo(HaveOccurred())
			Expect(infoData).To(Equal(staticTokenInfoData))
		})
	})

	Describe("#Marshal", func() {
		It("should properly marshal StaticTokenInfoData into StaticTokenJSONData", func() {
			data, err := staticTokenInfoData.Marshal()
			Expect(err).NotTo(HaveOccurred())
			Expect(data).To(Equal(staticTokenJSON))
		})
	})

	Describe("#TypeVersion", func() {
		It("should return the correct TypeVersion", func() {
			typeVersion := staticTokenInfoData.TypeVersion()
			Expect(typeVersion).To(Equal(StaticTokenDataType))
		})
	})

	Describe("#NewStaticTokenInfoData", func() {
		It("should return new StaticTokenInfoData from the passed tokens list", func() {
			tokens := map[string]string{
				"foo": "foo",
			}
			newStaticTokenInfoData := NewStaticTokenInfoData(tokens)
			Expect(newStaticTokenInfoData).To(Equal(staticTokenInfoData))
		})
	})
})
