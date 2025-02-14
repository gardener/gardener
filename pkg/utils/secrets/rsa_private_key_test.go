// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	"crypto/rand"
	"crypto/rsa"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/secrets"
)

var _ = Describe("RSA Private Key Secrets", func() {
	Describe("RSA Secret Configuration", func() {
		var rsaPrivateKeyConfig *RSASecretConfig

		BeforeEach(func() {
			rsaPrivateKeyConfig = &RSASecretConfig{
				Bits: 3072,
				Name: "rsa-secret",
			}
		})

		Describe("#Generate", func() {
			It("should properly generate RSAKeys object", func() {
				obj, err := rsaPrivateKeyConfig.Generate()
				Expect(err).NotTo(HaveOccurred())

				rsaSecret, ok := obj.(*RSAKeys)
				Expect(ok).To(BeTrue())

				Expect(rsaSecret.PrivateKey).NotTo(BeNil())
				Expect(*rsaSecret.PublicKey).To(Equal(rsaSecret.PrivateKey.PublicKey))

			})
			It("should generate ssh public key if specified in the config", func() {
				rsaPrivateKeyConfig.UsedForSSH = true
				obj, err := rsaPrivateKeyConfig.Generate()
				Expect(err).NotTo(HaveOccurred())

				rsaSecret, ok := obj.(*RSAKeys)
				Expect(ok).To(BeTrue())
				Expect(rsaSecret.OpenSSHAuthorizedKey).NotTo(BeNil())
			})
		})
	})

	Describe("RSAKeys Object", func() {
		var (
			rsaKeys *RSAKeys
			key     *rsa.PrivateKey
		)
		BeforeEach(func() {
			var err error
			key, err = rsa.GenerateKey(rand.Reader, 3072)
			Expect(err).NotTo(HaveOccurred())

			rsaKeys = &RSAKeys{
				PrivateKey:           key,
				OpenSSHAuthorizedKey: []byte("bar"),
			}
		})

		Describe("#SecretData", func() {
			It("should properly return secret data", func() {
				secretData := map[string][]byte{
					DataKeyRSAPrivateKey:     utils.EncodePrivateKey(key),
					DataKeySSHAuthorizedKeys: []byte("bar"),
				}
				Expect(rsaKeys.SecretData()).To(Equal(secretData))
			})
		})
	})
})
