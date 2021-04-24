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

package networkpolicies

import (
	"net"

	networkingv1 "k8s.io/api/networking/v1"
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

// ToNetworkPolicyPeersWithExceptions returns a list of networkingv1.NetworkPolicyPeers whose ipBlock.cidr points to
// `networks` and whose ipBlock.except points to `except`.
func ToNetworkPolicyPeersWithExceptions(networks []net.IPNet, except ...string) ([]networkingv1.NetworkPolicyPeer, error) {
	var result []networkingv1.NetworkPolicyPeer

	for _, n := range networks {
		excluded, err := excludeBlock(&n, except...)
		if err != nil {
			return nil, err
		}

		result = append(result, networkingv1.NetworkPolicyPeer{
			IPBlock: &networkingv1.IPBlock{
				CIDR:   n.String(),
				Except: excluded,
			},
		})
	}

	return result, nil
}

// NetworkPolicyPeersWithExceptions returns a list of networkingv1.NetworkPolicyPeers whose ipBlock.cidr points to
// `networks` and whose ipBlock.except points to `except`.
func NetworkPolicyPeersWithExceptions(networks []string, except ...string) ([]networkingv1.NetworkPolicyPeer, error) {
	var ipNets []net.IPNet

	for _, n := range networks {
		_, net, err := net.ParseCIDR(string(n))
		if err != nil {
			return nil, err
		}

		ipNets = append(ipNets, *net)
	}

	return ToNetworkPolicyPeersWithExceptions(ipNets, except...)
}

func excludeBlock(parentBlock *net.IPNet, cidrs ...string) ([]string, error) {
	var matchedCIDRs []string

	for _, cidr := range cidrs {
		ip, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return matchedCIDRs, err
		}

		if parentBlock.Contains(ip) && !ipNet.Contains(parentBlock.IP) {
			matchedCIDRs = append(matchedCIDRs, cidr)
		}
	}

	return matchedCIDRs, nil
}
