// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	. "github.com/gardener/gardener/pkg/utils/secrets"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CertificateBundle Secrets", func() {
	var (
		name  = "bundle"
		cert1 = []byte("cert1")
		cert2 = []byte("cert2")
	)

	Describe("Configuration", func() {
		var config *CertificateBundleSecretConfig

		BeforeEach(func() {
			config = &CertificateBundleSecretConfig{
				Name:            name,
				CertificatePEMs: [][]byte{cert1, cert2},
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

				bundle, ok := obj.(*CertificateBundle)
				Expect(ok).To(BeTrue())

				Expect(bundle.Name).To(Equal(name))
				Expect(bundle.Bundle).To(Equal(append(cert1, cert2...)))
			})
		})
	})

	Describe("Bundle", func() {
		var bundle *CertificateBundle

		BeforeEach(func() {
			bundle = &CertificateBundle{
				Name:   name,
				Bundle: append(cert1, cert2...),
			}
		})

		Describe("#SecretData", func() {
			It("should return the correct data map", func() {
				Expect(bundle.SecretData()).To(Equal(map[string][]byte{
					"bundle.crt": bundle.Bundle,
				}))
			})
		})
	})
})
