// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	. "github.com/gardener/gardener/pkg/utils/secrets"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BasicAuth InfoData", func() {
	var (
		basicAuthJSON     = []byte(`{"password":"foo"}`)
		basicAuthInfoData = &BasicAuthInfoData{
			Password: "foo",
		}
	)

	Describe("#UnmarshalBasicAuth", func() {
		It("should properly unmarshal BasicAuthJSONData into BasicAuthInfoData", func() {
			infoData, err := UnmarshalBasicAuth(basicAuthJSON)
			Expect(err).NotTo(HaveOccurred())
			Expect(infoData).To(Equal(basicAuthInfoData))
		})
	})

	Describe("#Marshal", func() {
		It("should properly marshal BaiscAuthInfoData into BasicAuthJSONData", func() {
			data, err := basicAuthInfoData.Marshal()
			Expect(err).NotTo(HaveOccurred())
			Expect(data).To(Equal(basicAuthJSON))
		})
	})

	Describe("#TypeVersion", func() {
		It("should return the correct TypeVersion", func() {
			typeVersion := basicAuthInfoData.TypeVersion()
			Expect(typeVersion).To(Equal(BasicAuthDataType))
		})
	})

	Describe("#NewBasicAuthInfoData", func() {
		It("should return new BasicAuthInfoData from the passed password", func() {
			newBasicAuthInfoData := NewBasicAuthInfoData("foo")
			Expect(newBasicAuthInfoData).To(Equal(basicAuthInfoData))
		})
	})
})
