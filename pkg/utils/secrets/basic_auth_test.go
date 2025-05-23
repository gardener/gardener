// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/bcrypt"

	. "github.com/gardener/gardener/pkg/utils/secrets"
)

var _ = Describe("Basic Auth Secrets", func() {
	Describe("Configuration", func() {
		var basicAuthConfiguration *BasicAuthSecretConfig

		BeforeEach(func() {
			basicAuthConfiguration = &BasicAuthSecretConfig{
				Name:           "basic-auth",
				Format:         BasicAuthFormatNormal,
				Username:       "admin",
				PasswordLength: 32,
			}
		})

		Describe("#Generate", func() {
			It("should properly generate Basic Auth Object", func() {
				obj, err := basicAuthConfiguration.Generate()
				Expect(err).NotTo(HaveOccurred())

				basicAuth, ok := obj.(*BasicAuth)
				Expect(ok).To(BeTrue())

				password := strings.TrimPrefix(string(basicAuth.SecretData()[DataKeyAuth]), basicAuthConfiguration.Username+":")
				Expect(bcrypt.CompareHashAndPassword([]byte(password), []byte(basicAuth.Password))).To(Succeed())
			})
		})

		Describe("#SecretData", func() {
			It("should properly return secret data if format is BasicAuthFormatNormal", func() {
				obj, err := basicAuthConfiguration.Generate()
				Expect(err).NotTo(HaveOccurred())

				data := obj.SecretData()
				password := strings.TrimPrefix(string(data[DataKeyAuth]), string(data[DataKeyUserName])+":")
				Expect(bcrypt.CompareHashAndPassword([]byte(password), data[DataKeyPassword])).To(Succeed())
			})
		})
	})
})
