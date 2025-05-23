// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/secrets"
)

var _ = Describe("VPN TLS Auth Secrets", func() {
	Describe("VPN TLS Auth Secret Configuration", func() {
		var vpnTLSAuthConfig *VPNTLSAuthConfig

		BeforeEach(func() {
			vpnTLSAuthConfig = &VPNTLSAuthConfig{
				Name: "vpn-secret",
				VPNTLSAuthKeyGenerator: func() ([]byte, error) {
					return []byte("foo"), nil
				},
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
