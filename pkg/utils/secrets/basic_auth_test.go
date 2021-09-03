// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package secrets_test

import (
	. "github.com/gardener/gardener/pkg/utils/secrets"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Basic Auth Secrets", func() {
	Describe("Basic Auth Configuration", func() {
		compareCurrentAndExpectedBasicAuth := func(current DataInterface, expected *BasicAuth, comparePasswords bool) {
			basicAuth, ok := current.(*BasicAuth)
			Expect(ok).To(BeTrue())

			Expect(basicAuth.Name).To(Equal(expected.Name))
			Expect(basicAuth.Format).To(Equal(expected.Format))
			Expect(basicAuth.Username).To(Equal(expected.Username))

			if comparePasswords {
				Expect(basicAuth.Password).To(Equal(expected.Password))
			} else {
				Expect(basicAuth.Password).NotTo(Equal(""))
			}
		}

		var (
			expectedBasicAuthObject *BasicAuth
			basicAuthConfiguration  *BasicAuthSecretConfig
			basicAuthInfoData       *BasicAuthInfoData
		)

		BeforeEach(func() {
			basicAuthConfiguration = &BasicAuthSecretConfig{
				Name:           "basic-auth",
				Format:         BasicAuthFormatCSV,
				Username:       "admin",
				PasswordLength: 32,
			}

			basicAuthInfoData = &BasicAuthInfoData{
				Password: "foo",
			}

			expectedBasicAuthObject = &BasicAuth{
				Name:     "basic-auth",
				Format:   BasicAuthFormatCSV,
				Username: "admin",
				Password: "foo",
			}
		})

		Describe("#Generate", func() {
			It("should properly generate Basic Auth Object", func() {
				obj, err := basicAuthConfiguration.Generate()
				Expect(err).NotTo(HaveOccurred())
				compareCurrentAndExpectedBasicAuth(obj, expectedBasicAuthObject, false)
			})
		})

		Describe("#GenerateInfoData", func() {
			It("should generate correct BasicAuth InfoData", func() {
				obj, err := basicAuthConfiguration.GenerateInfoData()
				Expect(err).NotTo(HaveOccurred())

				Expect(obj.TypeVersion()).To(Equal(BasicAuthDataType))

				basicAuthInfoData, ok := obj.(*BasicAuthInfoData)
				Expect(ok).To(BeTrue())

				Expect(basicAuthInfoData.Password).NotTo(Equal(""))
			})
		})

		Describe("#GenerateFromInfoData", func() {
			It("should properly load Basic Auth object from BasicAuthInfoData", func() {
				obj, err := basicAuthConfiguration.GenerateFromInfoData(basicAuthInfoData)
				Expect(err).NotTo(HaveOccurred())

				compareCurrentAndExpectedBasicAuth(obj, expectedBasicAuthObject, false)
			})
		})

		DescribeTable("#LoadFromSecretData", func(secretData map[string][]byte, format, password string) {
			if format == string(BasicAuthFormatNormal) {
				basicAuthConfiguration.Format = BasicAuthFormatNormal
			} else if format == string(BasicAuthFormatCSV) {
				basicAuthConfiguration.Format = BasicAuthFormatCSV
			}
			obj, err := basicAuthConfiguration.LoadFromSecretData(secretData)
			Expect(err).NotTo(HaveOccurred())

			Expect(obj.TypeVersion()).To(Equal(BasicAuthDataType))

			basicAuthInfoData, ok := obj.(*BasicAuthInfoData)
			Expect(ok).To(BeTrue())

			Expect(basicAuthInfoData.Password).To(Equal(password))
		},
			Entry("Load from csv secret data", map[string][]byte{
				DataKeyCSV: []byte("foo,admin,admin,group"),
			}, string(BasicAuthFormatCSV), "foo"),
			Entry("Load from normal format secret data", map[string][]byte{
				DataKeyUserName: []byte("admin"),
				DataKeyPassword: []byte("foo"),
			}, string(BasicAuthFormatNormal), "foo"))
	})

	Describe("Basic Auth Object", func() {
		var (
			basicAuth                *BasicAuth
			expectedNormalFormatData map[string][]byte
			expectedCSVFormatData    map[string][]byte
		)
		BeforeEach(func() {
			basicAuth = &BasicAuth{
				Name:     "basicauth",
				Username: "admin",
				Password: "foo",
				Format:   BasicAuthFormatCSV,
			}

			expectedNormalFormatData = map[string][]byte{
				DataKeyUserName: []byte("admin"),
				DataKeyPassword: []byte("foo"),
				DataKeyCSV:      []byte("foo,admin,admin,system:masters"),
			}

			expectedCSVFormatData = map[string][]byte{
				DataKeyCSV: []byte("foo,admin,admin,system:masters"),
			}
		})

		Describe("#SecretData", func() {
			It("should properly return secret data if format is BasicAuthFormatNormal", func() {
				basicAuth.Format = BasicAuthFormatNormal
				data := basicAuth.SecretData()
				Expect(data).To(Equal(expectedNormalFormatData))
			})
			It("should properly return secret data if format is CSV", func() {
				data := basicAuth.SecretData()
				Expect(data).To(Equal(expectedCSVFormatData))
			})
		})

		Describe("#LoadBasicAuthFromCSV", func() {
			It("should properly load BasicAuth object from CSV data", func() {
				obj, err := LoadBasicAuthFromCSV("basicauth", expectedCSVFormatData[DataKeyCSV])
				Expect(err).NotTo(HaveOccurred())
				Expect(obj.Username).To(Equal("admin"))
				Expect(obj.Password).To(Equal("foo"))
				Expect(obj.Name).To(Equal("basicauth"))
			})
		})
	})
})
