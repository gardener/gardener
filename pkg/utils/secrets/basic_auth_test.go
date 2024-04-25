// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/secrets"
)

var _ = Describe("Basic Auth Secrets", func() {
	Describe("Basic Auth Configuration", func() {
		compareCurrentAndExpectedBasicAuth := func(current DataInterface, expected *BasicAuth, comparePasswords bool) {
			basicAuth, ok := current.(*BasicAuth)
			Expect(ok).To(BeTrue())

			Expect(basicAuth.Name).To(Equal(expected.Name))
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
		)

		BeforeEach(func() {
			basicAuthConfiguration = &BasicAuthSecretConfig{
				Name:           "basic-auth",
				Format:         BasicAuthFormatNormal,
				Username:       "admin",
				PasswordLength: 32,
			}

			expectedBasicAuthObject = &BasicAuth{
				Name:     "basic-auth",
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
	})

	Describe("Basic Auth Object", func() {
		var (
			basicAuth                *BasicAuth
			expectedNormalFormatData map[string][]byte
		)
		BeforeEach(func() {
			basicAuth = &BasicAuth{
				Name:     "basicauth",
				Username: "admin",
				Password: "foo",
			}

			expectedNormalFormatData = map[string][]byte{
				DataKeyUserName: []byte("admin"),
				DataKeyPassword: []byte("foo"),
				DataKeySHA1Auth: []byte("admin:{SHA}C+7Hteo/D9vJXQ3UfzxbwnXaijM="),
			}
		})

		Describe("#SecretData", func() {
			It("should properly return secret data if format is BasicAuthFormatNormal", func() {
				data := basicAuth.SecretData()
				Expect(data).To(Equal(expectedNormalFormatData))
			})
		})
	})
})
