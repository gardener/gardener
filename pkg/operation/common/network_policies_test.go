// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package common_test

import (
	"net"

	. "github.com/gardener/gardener/pkg/operation/common"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("networkpolicies", func() {
	Describe("#AllPrivateNetworkBlocks", func() {
		It("should contain correct CIDRs", func() {
			result := AllPrivateNetworkBlocks()
			_, block8, _ := net.ParseCIDR("10.0.0.0/8")
			_, block12, _ := net.ParseCIDR("172.16.0.0/12")
			_, block16, _ := net.ParseCIDR("192.168.0.0/16")
			_, carrierGradeBlock, _ := net.ParseCIDR("100.64.0.0/10")
			Expect(result).To(ConsistOf(*block8, *block12, *block16, *carrierGradeBlock))
		})
	})

	Describe("#ToExceptNetworks", func() {
		It("should return correct result", func() {
			result, err := ToExceptNetworks(AllPrivateNetworkBlocks(), "10.10.0.0/24", "172.16.1.0/24", "192.168.1.0/24", "100.64.1.0/24")
			expectedResult := []interface{}{
				map[string]interface{}{
					"network": "10.0.0.0/8",
					"except":  []string{"10.10.0.0/24"},
				},
				map[string]interface{}{
					"network": "172.16.0.0/12",
					"except":  []string{"172.16.1.0/24"},
				},
				map[string]interface{}{
					"network": "192.168.0.0/16",
					"except":  []string{"192.168.1.0/24"},
				},
				map[string]interface{}{
					"network": "100.64.0.0/10",
					"except":  []string{"100.64.1.0/24"},
				},
			}

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ConsistOf(expectedResult))
		})
	})
})
