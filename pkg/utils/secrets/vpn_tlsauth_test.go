// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VPN TLS Auth Secrets", func() {
	Describe("VPN TLS Auth Secret Configuration", func() {
		var (
			vpnTLSAuthConfig   *VPNTLSAuthConfig
			privateKeyInfoData *PrivateKeyInfoData
		)

		BeforeEach(func() {
			vpnTLSAuthConfig = &VPNTLSAuthConfig{
				Name: "vpn-secret",
				VPNTLSAuthKeyGenerator: func() ([]byte, error) {
					return []byte("foo"), nil
				},
			}

			privateKeyInfoData = &PrivateKeyInfoData{
				PrivateKey: []byte("foo"),
			}
		})

		Describe("#Generate", func() {
			It("should properly generate VPNTLSAuth object", func() {
				obj, err := vpnTLSAuthConfig.Generate()
				Expect(err).NotTo(HaveOccurred())

				vpnTLSAuth, ok := obj.(*VPNTLSAuth)
				Expect(ok).To(BeTrue())

				Expect(vpnTLSAuth.TLSAuthKey).ToNot(BeEmpty())
			})
		})

		Describe("#GenerateInfoData", func() {
			It("should generate correct PrivateKey InfoData", func() {
				obj, err := vpnTLSAuthConfig.GenerateInfoData()
				Expect(err).NotTo(HaveOccurred())

				Expect(obj.TypeVersion()).To(Equal(PrivateKeyDataType))

				currentStaticTokenInfoData, ok := obj.(*PrivateKeyInfoData)
				Expect(ok).To(BeTrue())

				Expect(currentStaticTokenInfoData.PrivateKey).ToNot(BeEmpty())
			})
		})

		Describe("#GenerateFromInfoData", func() {
			It("should properly load VPNTLSAuth object from PrivateKeyInfoData", func() {
				obj, err := vpnTLSAuthConfig.GenerateFromInfoData(privateKeyInfoData)
				Expect(err).NotTo(HaveOccurred())

				vpnTLSAuth, ok := obj.(*VPNTLSAuth)
				Expect(ok).To(BeTrue())

				Expect(vpnTLSAuth.TLSAuthKey).To(Equal(privateKeyInfoData.PrivateKey))
			})
		})

		Describe("#LoadFromSecretData", func() {
			It("should properly load PrivateKeyInfoData from vpn tls auth secret data", func() {
				secretData := map[string][]byte{
					DataKeyVPNTLSAuth: []byte("foo"),
				}
				obj, err := vpnTLSAuthConfig.LoadFromSecretData(secretData)
				Expect(err).NotTo(HaveOccurred())

				currentVPNTLSAuthInfoData, ok := obj.(*PrivateKeyInfoData)
				Expect(ok).To(BeTrue())

				Expect(currentVPNTLSAuthInfoData).To(Equal(privateKeyInfoData))
			})
		})
	})

	Describe("VPNTLSAuth Object", func() {
		var (
			vpnTLSAuth *VPNTLSAuth
		)
		BeforeEach(func() {
			vpnTLSAuth = &VPNTLSAuth{
				TLSAuthKey: []byte("foo"),
			}
		})

		Describe("#SecretData", func() {
			It("should properly return secret data", func() {
				secretData := map[string][]byte{
					DataKeyVPNTLSAuth: []byte("foo"),
				}
				Expect(vpnTLSAuth.SecretData()).To(Equal(secretData))
			})
		})
	})
})
