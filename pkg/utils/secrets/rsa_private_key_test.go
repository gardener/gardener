// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
				Bits: 16,
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
			key, err = rsa.GenerateKey(rand.Reader, 16)
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
