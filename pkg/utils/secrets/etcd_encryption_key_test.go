// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

				Expect(etcdEncryptionKey.Name).To(Equal(name))
				Expect(etcdEncryptionKey.Key).To(Equal("key-62135596800"))
				Expect(etcdEncryptionKey.Secret).To(Equal("_________________"))
			})
		})

		Describe("#SecretData", func() {
			It("should return the correct data map", func() {
				obj, err := config.Generate()
				Expect(err).NotTo(HaveOccurred())

				etcdEncryptionKey, ok := obj.(*ETCDEncryptionKey)
				Expect(ok).To(BeTrue())

				Expect(etcdEncryptionKey.SecretData()).To(Equal(map[string][]byte{
					"key":    []byte("key-62135596800"),
					"secret": []byte("_________________"),
				}))
			})
		})
	})
})
