// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/secrets"
)

var _ = Describe("Bundle Secrets", func() {
	var (
		name   = "bundle"
		entry1 = []byte("entry1")
		entry2 = []byte("entry2")
	)

	Describe("CertificateBundleSecretConfig", func() {
		var config *CertificateBundleSecretConfig

		BeforeEach(func() {
			config = &CertificateBundleSecretConfig{
				Name:            name,
				CertificatePEMs: [][]byte{entry1, entry2},
			}
		})

		Describe("#GetName", func() {
			It("should return the name", func() {
				Expect(config.GetName()).To(Equal(name))
			})
		})

		Describe("#Generate", func() {
			It("should generate the bundle", func() {
				obj, err := config.Generate()
				Expect(err).NotTo(HaveOccurred())

				bundle, ok := obj.(*Bundle)
				Expect(ok).To(BeTrue())

				Expect(bundle.Name).To(Equal(name))
				Expect(bundle.Bundle).To(Equal(append(entry1, entry2...)))

				Expect(bundle.SecretData()).To(Equal(map[string][]byte{"bundle.crt": bundle.Bundle}))
			})
		})
	})

	Describe("RSAPrivateKeyBundleSecretConfig", func() {
		var config *RSAPrivateKeyBundleSecretConfig

		BeforeEach(func() {
			config = &RSAPrivateKeyBundleSecretConfig{
				Name:           name,
				PrivateKeyPEMs: [][]byte{entry1, entry2},
			}
		})

		Describe("#GetName", func() {
			It("should return the name", func() {
				Expect(config.GetName()).To(Equal(name))
			})
		})

		Describe("#Generate", func() {
			It("should generate the bundle", func() {
				obj, err := config.Generate()
				Expect(err).NotTo(HaveOccurred())

				bundle, ok := obj.(*Bundle)
				Expect(ok).To(BeTrue())

				Expect(bundle.Name).To(Equal(name))
				Expect(bundle.Bundle).To(Equal(append(entry1, entry2...)))

				Expect(bundle.SecretData()).To(Equal(map[string][]byte{"bundle.key": bundle.Bundle}))
			})
		})
	})

	Describe("Bundle", func() {
		var (
			bundle  *Bundle
			dataKey = "some.key"
		)

		BeforeEach(func() {
			bundle = &Bundle{
				Name:        name,
				Bundle:      append(entry1, entry2...),
				DataKeyName: dataKey,
			}
		})

		Describe("#SecretData", func() {
			It("should return the correct data map", func() {
				Expect(bundle.SecretData()).To(Equal(map[string][]byte{
					dataKey: bundle.Bundle,
				}))
			})
		})
	})
})
