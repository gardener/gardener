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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/secrets"
)

var _ = Describe("Certificate Secrets", func() {
	Describe("Certificate Configuration", func() {
		var certificateConfig *CertificateSecretConfig

		BeforeEach(func() {
			certificateConfig = &CertificateSecretConfig{
				Name:       "ca",
				CommonName: "metrics-server",
				CertType:   CACert,
			}
		})

		Describe("#Generate", func() {
			It("should properly generate CA Certificate Object", func() {
				obj, err := certificateConfig.Generate()
				Expect(err).NotTo(HaveOccurred())

				certificate, ok := obj.(*Certificate)
				Expect(ok).To(BeTrue())

				Expect(certificate.PrivateKeyPEM).NotTo(BeNil())
				Expect(certificate.CertificatePEM).NotTo(BeNil())
				Expect(certificate.PrivateKey).NotTo(BeNil())
				Expect(certificate.Certificate).NotTo(BeNil())
				Expect(certificate.CA).To(BeNil())
			})
		})
	})

	Describe("Certificate Object", func() {
		var (
			certificate *Certificate
		)
		BeforeEach(func() {
			certificate = &Certificate{
				PrivateKeyPEM:  []byte("foo"),
				CertificatePEM: []byte("bar"),
			}
		})

		Describe("#SecretData", func() {
			It("should properly return secret data if certificate type is CA", func() {
				Expect(certificate.SecretData()).To(Equal(map[string][]byte{
					DataKeyPrivateKeyCA:  []byte("foo"),
					DataKeyCertificateCA: []byte("bar"),
				}))
			})

			It("should properly return secret data if certificate type is server, client or both", func() {
				certificate.CA = &Certificate{CertificatePEM: []byte("ca")}

				Expect(certificate.SecretData()).To(Equal(map[string][]byte{
					DataKeyPrivateKey:    []byte("foo"),
					DataKeyCertificate:   []byte("bar"),
					DataKeyCertificateCA: []byte("ca"),
				}))
			})

			It("should properly return secret data if certificate type is server, client or both w/o publishing CA", func() {
				certificate.CA = &Certificate{CertificatePEM: []byte("ca")}
				certificate.SkipPublishingCACertificate = true

				Expect(certificate.SecretData()).To(Equal(map[string][]byte{
					DataKeyPrivateKey:  []byte("foo"),
					DataKeyCertificate: []byte("bar"),
				}))
			})
		})
	})
})
