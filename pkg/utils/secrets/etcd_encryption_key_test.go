// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/secrets"
)

var _ = Describe("Etcd Encryption Key Secrets", func() {
	var (
		name         = "etcd encryption key"
		secretLength = 17
	)

	Describe("Configuration", func() {
		var config *ETCDEncryptionKeySecretConfig

		BeforeEach(func() {
			config = &ETCDEncryptionKeySecretConfig{
				Name:         name,
				SecretLength: secretLength,
			}
		})

		Describe("#GetName", func() {
			It("should return the name", func() {
				Expect(config.GetName()).To(Equal(name))
			})
		})

		Describe("#Generate", func() {
			It("should generate the key", func() {
				obj, err := config.Generate()
				Expect(err).NotTo(HaveOccurred())

				etcdEncryptionKey, ok := obj.(*ETCDEncryptionKey)
				Expect(ok).To(BeTrue())

				Expect(etcdEncryptionKey.KeyName).To(Equal("key-62135596800"))
				Expect(etcdEncryptionKey.Secret).To(Equal([]byte("_________________")))
			})
		})

		Describe("#SecretData", func() {
			It("should return the correct data map", func() {
				obj, err := config.Generate()
				Expect(err).NotTo(HaveOccurred())

				etcdEncryptionKey, ok := obj.(*ETCDEncryptionKey)
				Expect(ok).To(BeTrue())

				Expect(etcdEncryptionKey.SecretData()).To(Equal(map[string][]byte{
					"key":      []byte("key-62135596800"),
					"secret":   []byte("_________________"),
					"encoding": []byte("none"),
				}))
			})
		})
	})
})
