// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/secrets"
)

var _ = Describe("Static Token Secrets", func() {
	Describe("StaticToken Secret Configuration", func() {
		var (
			staticTokenConfig *StaticTokenSecretConfig
			username          = "bar"
		)

		BeforeEach(func() {
			staticTokenConfig = &StaticTokenSecretConfig{
				Name: "static-token",
				Tokens: map[string]TokenConfig{
					username: {
						Username: username,
						UserID:   "foo",
						Groups:   []string{"group"},
					},
				},
			}
		})

		Describe("#Generate", func() {
			It("should properly generate RSAKeys object", func() {
				obj, err := staticTokenConfig.Generate()
				Expect(err).NotTo(HaveOccurred())

				staticToken, ok := obj.(*StaticToken)
				Expect(ok).To(BeTrue())

				Expect(staticToken.Tokens).To(HaveLen(1))
				Expect(staticToken.Tokens[0].Token).ToNot(Equal(""))
				Expect(staticToken.Tokens[0].Username).To(Equal(staticTokenConfig.Tokens[username].Username))
				Expect(staticToken.Tokens[0].UserID).To(Equal(staticTokenConfig.Tokens[username].UserID))
				Expect(staticToken.Tokens[0].Groups).To(Equal(staticTokenConfig.Tokens[username].Groups))
			})
		})
	})

	Describe("StaticToken Object", func() {
		var staticToken *StaticToken

		BeforeEach(func() {
			staticToken = &StaticToken{
				Tokens: []Token{
					{
						Token:    "foo",
						Username: "foo",
						UserID:   "bar",
						Groups:   []string{"group"},
					},
				},
			}
		})

		Describe("#SecretData", func() {
			It("should properly return secret data", func() {
				secretData := map[string][]byte{
					DataKeyStaticTokenCSV: []byte("foo,foo,bar,group"),
				}
				Expect(staticToken.SecretData()).To(Equal(secretData))
			})
		})
	})
})
