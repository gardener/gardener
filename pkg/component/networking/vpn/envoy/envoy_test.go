// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package envoy_test

import (
	"net"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/component/networking/vpn/envoy"
)

var _ = Describe("Envoy", func() {
	Describe("#GetEnvoyConfig", func() {
		It("should require a client certificate on the proxy listener", func() {
			config, err := envoy.GetEnvoyConfig([]net.IPNet{{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(24, 32)}})
			Expect(err).NotTo(HaveOccurred())

			Expect(config).To(ContainSubstring("require_client_certificate: true"))
		})

		It("should restrict destinations via resolved_address_filter on both dns_cache_config copies", func() {
			config, err := envoy.GetEnvoyConfig([]net.IPNet{
				{IP: net.ParseIP("10.0.2.0"), Mask: net.CIDRMask(24, 32)},
				{IP: net.ParseIP("2001:db8:3::"), Mask: net.CIDRMask(48, 128)},
			})
			Expect(err).NotTo(HaveOccurred())

			// The named DNS cache appears twice (HTTP filter and cluster); both must carry the filter.
			Expect(strings.Count(config, "resolved_address_filter:")).To(Equal(2))
			Expect(strings.Count(config, "invert_match: true")).To(Equal(2))
			Expect(strings.Count(config, `address_prefix: "10.0.2.0"`)).To(Equal(2))
			Expect(strings.Count(config, "prefix_len: 24")).To(Equal(2))
			Expect(strings.Count(config, `address_prefix: "2001:db8:3::"`)).To(Equal(2))
			Expect(strings.Count(config, "prefix_len: 48")).To(Equal(2))
		})

		It("should fail closed (deny all) when no CIDRs are configured", func() {
			config, err := envoy.GetEnvoyConfig(nil)
			Expect(err).NotTo(HaveOccurred())

			// invert_match with an empty allow-list removes every resolved address => deny all.
			Expect(strings.Count(config, "resolved_address_filter:")).To(Equal(2))
			Expect(strings.Count(config, "invert_match: true")).To(Equal(2))
			Expect(strings.Count(config, "ranges: []")).To(Equal(2))
			Expect(config).NotTo(ContainSubstring("address_prefix:"))
		})
	})
})
