// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
			It("should properly return secret data for CA certificates", func() {
				Expect(certificate.SecretData()).To(Equal(map[string][]byte{
					DataKeyPrivateKeyCA:  []byte("foo"),
					DataKeyCertificateCA: []byte("bar"),
				}))
			})

			It("should properly return secret data for non-CA certificates", func() {
				certificate.CA = &Certificate{CertificatePEM: []byte("ca")}

				Expect(certificate.SecretData()).To(Equal(map[string][]byte{
					DataKeyPrivateKey:    []byte("foo"),
					DataKeyCertificate:   []byte("bar"),
					DataKeyCertificateCA: []byte("ca"),
				}))
			})

			It("should properly return secret data for non-CA certificates w/o publishing CA", func() {
				certificate.CA = &Certificate{CertificatePEM: []byte("ca")}
				certificate.SkipPublishingCACertificate = true

				Expect(certificate.SecretData()).To(Equal(map[string][]byte{
					DataKeyPrivateKey:  []byte("foo"),
					DataKeyCertificate: []byte("bar"),
				}))
			})

			Context("w/ CA included in chain", func() {
				BeforeEach(func() {
					certificate.CA = &Certificate{CertificatePEM: []byte("ca")}
					certificate.IncludeCACertificateInServerChain = true
					certificate.SkipPublishingCACertificate = true
				})

				It("should properly return secret data for server certificates", func() {
					certificate.CertType = ServerCert

					Expect(certificate.SecretData()).To(Equal(map[string][]byte{
						DataKeyPrivateKey:  []byte("foo"),
						DataKeyCertificate: []byte("barca"),
					}))
				})

				It("should properly return secret data for client certificates", func() {
					certificate.CertType = ClientCert

					Expect(certificate.SecretData()).To(Equal(map[string][]byte{
						DataKeyPrivateKey:  []byte("foo"),
						DataKeyCertificate: []byte("bar"),
					}))
				})
			})
		})
	})
})
