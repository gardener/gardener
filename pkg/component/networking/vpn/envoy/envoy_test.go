// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package envoy_test

import (
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	"github.com/gardener/gardener/pkg/component/networking/vpn/envoy"
)

var _ = Describe("Envoy", func() {
	Describe("#GetEnvoyConfig", func() {
		// dnsCacheConfigs returns the dns_cache_config block from both the dynamic_forward_proxy HTTP
		// filter and the dynamic_forward_proxy cluster. The destination filter must be present and
		// identical on both (they share the same named DNS cache); if only one carried the filter,
		// Envoy would reject the config, so the test asserts on both.
		dnsCacheConfigs := func(config string) []map[string]any {
			var root map[string]any
			ExpectWithOffset(1, yaml.Unmarshal([]byte(config), &root)).To(Succeed())

			var out []map[string]any
			resources := root["static_resources"].(map[string]any)

			// HTTP filter copy.
			for _, l := range resources["listeners"].([]any) {
				for _, fc := range l.(map[string]any)["filter_chains"].([]any) {
					filters, ok := fc.(map[string]any)["filters"].([]any)
					if !ok {
						continue
					}
					for _, f := range filters {
						tc, ok := f.(map[string]any)["typed_config"].(map[string]any)
						if !ok || tc["stat_prefix"] != "ingress_http" {
							continue
						}
						for _, hf := range tc["http_filters"].([]any) {
							htc, ok := hf.(map[string]any)["typed_config"].(map[string]any)
							if ok && htc["dns_cache_config"] != nil {
								out = append(out, htc["dns_cache_config"].(map[string]any))
							}
						}
					}
				}
			}

			// Cluster copy.
			for _, c := range resources["clusters"].([]any) {
				ct, ok := c.(map[string]any)["cluster_type"].(map[string]any)
				if !ok {
					continue
				}
				if tc, ok := ct["typed_config"].(map[string]any); ok && tc["dns_cache_config"] != nil {
					out = append(out, tc["dns_cache_config"].(map[string]any))
				}
			}
			return out
		}

		It("should require a client certificate on the proxy listener", func() {
			config, err := envoy.GetEnvoyConfig(nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(config).To(ContainSubstring("require_client_certificate: true"))
		})

		It("should NOT use a network RBAC filter (it matches the downstream listener IP, not the CONNECT target)", func() {
			config, err := envoy.GetEnvoyConfig([]net.IPNet{{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(24, 32)}})
			Expect(err).NotTo(HaveOccurred())

			Expect(config).NotTo(ContainSubstring("filters.network.rbac"))
			Expect(config).NotTo(ContainSubstring("connect_authz"))
		})

		It("should restrict destinations via resolved_address_filter on BOTH dns_cache_config copies", func() {
			config, err := envoy.GetEnvoyConfig([]net.IPNet{
				{IP: net.ParseIP("10.0.2.0"), Mask: net.CIDRMask(24, 32)},
				{IP: net.ParseIP("10.0.1.0"), Mask: net.CIDRMask(24, 32)},
			})
			Expect(err).NotTo(HaveOccurred())

			caches := dnsCacheConfigs(config)
			// One copy on the HTTP filter, one on the cluster.
			Expect(caches).To(HaveLen(2))

			for _, cache := range caches {
				raf, ok := cache["resolved_address_filter"].(map[string]any)
				Expect(ok).To(BeTrue(), "resolved_address_filter missing from a dns_cache_config")

				// invert_match: true => the listed CIDRs are the allow-list; everything else is dropped.
				Expect(raf["invert_match"]).To(BeTrue())

				ranges := raf["ranges"].([]any)
				Expect(ranges).To(HaveLen(2))
				prefixes := map[string]any{}
				for _, r := range ranges {
					rm := r.(map[string]any)
					prefixes[rm["address_prefix"].(string)] = rm["prefix_len"]
				}
				Expect(prefixes).To(HaveKeyWithValue("10.0.2.0", BeNumerically("==", 24)))
				Expect(prefixes).To(HaveKeyWithValue("10.0.1.0", BeNumerically("==", 24)))
			}
		})

		It("should render dual-stack CIDRs with correct prefix lengths", func() {
			config, err := envoy.GetEnvoyConfig([]net.IPNet{
				{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(24, 32)},
				{IP: net.ParseIP("2001:db8:3::"), Mask: net.CIDRMask(48, 128)},
			})
			Expect(err).NotTo(HaveOccurred())

			for _, cache := range dnsCacheConfigs(config) {
				raf := cache["resolved_address_filter"].(map[string]any)
				prefixes := map[string]any{}
				for _, r := range raf["ranges"].([]any) {
					rm := r.(map[string]any)
					prefixes[rm["address_prefix"].(string)] = rm["prefix_len"]
				}
				Expect(prefixes).To(HaveKeyWithValue("10.0.0.0", BeNumerically("==", 24)))
				Expect(prefixes).To(HaveKeyWithValue("2001:db8:3::", BeNumerically("==", 48)))
			}
		})

		It("should fail closed when no CIDRs are configured", func() {
			config, err := envoy.GetEnvoyConfig(nil)
			Expect(err).NotTo(HaveOccurred())

			caches := dnsCacheConfigs(config)
			Expect(caches).To(HaveLen(2))
			for _, cache := range caches {
				raf := cache["resolved_address_filter"].(map[string]any)
				// invert_match: true with an empty allow-list removes every resolved address => deny all.
				Expect(raf["invert_match"]).To(BeTrue())
				Expect(raf["ranges"]).To(BeEmpty())
			}
		})
	})
})
