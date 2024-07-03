// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package net_test

import (
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/net"
)

var _ = DescribeTable("#JoinByComma",
	func(cidrs []string, expected string) {
		var networks []net.IPNet
		for _, cidr := range cidrs {
			_, ipNet, err := net.ParseCIDR(cidr)
			Expect(err).ToNot(HaveOccurred())
			networks = append(networks, *ipNet)
		}
		Expect(JoinByComma(networks)).To(Equal(expected))
	},

	Entry("should work with nil list", nil, ""),
	Entry("should work with empty list", []string{}, ""),
	Entry("should work with single list entry", []string{"10.250.0.0/16"}, "10.250.0.0/16"),
	Entry("should concatenate CIDRs of the given networks with comma as separator",
		[]string{"10.250.0.0/16", "192.168.0.0/24", "2001:db8:1::/64", "2001:db8:2::/64"},
		"10.250.0.0/16,192.168.0.0/24,2001:db8:1::/64,2001:db8:2::/64"),
)

var _ = DescribeTable("#Join",
	func(cidrs []string, sep string, expected string) {
		var networks []net.IPNet
		for _, cidr := range cidrs {
			_, ipNet, err := net.ParseCIDR(cidr)
			Expect(err).ToNot(HaveOccurred())
			networks = append(networks, *ipNet)
		}
		Expect(Join(networks, sep)).To(Equal(expected))
	},

	Entry("should work with nil list", nil, ",", ""),
	Entry("should work with empty list", []string{}, " ", ""),
	Entry("should work with single list entry", []string{"10.250.0.0/16"}, "|", "10.250.0.0/16"),
	Entry("should concatenate CIDRs of the given networks with plus as separator",
		[]string{"10.250.0.0/16", "192.168.0.0/24", "2001:db8:1::/64", "2001:db8:2::/64"}, "+",
		"10.250.0.0/16+192.168.0.0/24+2001:db8:1::/64+2001:db8:2::/64"),
	Entry("should concatenate CIDRs of the given networks with *** as separator",
		[]string{"10.250.0.0/16", "192.168.0.0/24", "2001:db8:1::/64", "2001:db8:2::/64"}, "***",
		"10.250.0.0/16***192.168.0.0/24***2001:db8:1::/64***2001:db8:2::/64"),
)
