// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package common

import (
	"net"
)

// Private8BitBlock returns a private network (RFC1918) 10.0.0.0/8 IPv4 block
func Private8BitBlock() *net.IPNet {
	return &net.IPNet{IP: net.IP{10, 0, 0, 0}, Mask: net.CIDRMask(8, 32)}
}

// Private12BitBlock returns a private network (RFC1918) 172.16.0.0/12 IPv4 block
func Private12BitBlock() *net.IPNet {
	return &net.IPNet{IP: net.IP{172, 16, 0, 0}, Mask: net.CIDRMask(12, 32)}
}

// Private16BitBlock returns a private network (RFC1918) 192.168.0.0/16 IPv4 block
func Private16BitBlock() *net.IPNet {
	return &net.IPNet{IP: net.IP{192, 168, 0, 0}, Mask: net.CIDRMask(16, 32)}
}

// CarrierGradeNATBlock returns a Carrier-grade NAT (RFC6598) 100.64.0.0/10 IPv4 block
func CarrierGradeNATBlock() *net.IPNet {
	return &net.IPNet{IP: net.IP{100, 64, 0, 0}, Mask: net.CIDRMask(10, 32)}
}

// AllPrivateNetworkBlocks returns a list of all Private network (RFC1918) and
// Carrier-grade NAT (RFC6598) IPv4 blocks.
func AllPrivateNetworkBlocks() []net.IPNet {
	return []net.IPNet{
		*Private8BitBlock(),
		*Private12BitBlock(),
		*Private16BitBlock(),
		*CarrierGradeNATBlock(),
	}
}

// ToExceptNetworks returns a list of maps with `network` key containing one of `networks`
// and `except` key containgn list of `cidr` which are part of those CIDRs.
//
// Calling
// `ToExceptNetworks(AllPrivateNetworkBlocks(),"10.10.0.0/24","172.16.1.0/24","192.168.1.0/24","100.64.1.0/24")`
// produces:
//
// [
//		{"network": "10.0.0.0/8", "except": ["10.10.0.0/24"]},
//		{"network": "172.16.0.0/12", "except": ["172.16.1.0/24"]},
//		{"network": "192.168.0.0/16", "except": ["192.168.1.0/24"]},
//		{"network": "100.64.0.0/10", "except": ["100.64.1.0/24"]},
// ]
func ToExceptNetworks(networks []net.IPNet, except ...string) ([]interface{}, error) {
	result := []interface{}{}

	for _, n := range networks {
		excluded, err := excludeBlock(&n, except...)
		if err != nil {
			return nil, err
		}

		result = append(result, map[string]interface{}{
			"network": n.String(),
			"except":  excluded,
		})
	}
	return result, nil
}

// ExceptNetworks returns a list of maps with `network` key containing one of `networks`
// and `except` key containgn list of `cidr` which are part of those CIDRs.
//
// Calling
// `ExceptNetworks([]garden.CIDR{"10.0.0.0/8","172.16.0.0/12"},"10.10.0.0/24","172.16.1.0/24")`
// produces:
//
// [
//		{"network": "10.0.0.0/8", "except": ["10.10.0.0/24"]},
//		{"network": "172.16.0.0/12", "except": ["172.16.1.0/24"]},
// ]
func ExceptNetworks(networks []string, except ...string) ([]interface{}, error) {
	ipNets := []net.IPNet{}
	for _, n := range networks {
		_, net, err := net.ParseCIDR(string(n))
		if err != nil {
			return nil, err
		}
		ipNets = append(ipNets, *net)
	}
	return ToExceptNetworks(ipNets, except...)
}

func excludeBlock(parentBlock *net.IPNet, cidrs ...string) ([]string, error) {
	matchedCIDRs := []string{}

	for _, cidr := range cidrs {
		ip, ipNet, err := net.ParseCIDR(string(cidr))
		if err != nil {
			return matchedCIDRs, err
		}
		if parentBlock.Contains(ip) && !ipNet.Contains(parentBlock.IP) {
			matchedCIDRs = append(matchedCIDRs, cidr)
		}
	}
	return matchedCIDRs, nil
}
