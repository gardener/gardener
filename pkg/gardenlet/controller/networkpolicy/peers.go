// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package networkpolicy

import (
	"net"

	networkingv1 "k8s.io/api/networking/v1"
)

// private8BitBlock returns a private network (RFC1918) 10.0.0.0/8 IPv4 block.
func private8BitBlock() *net.IPNet {
	return &net.IPNet{IP: net.IP{10, 0, 0, 0}, Mask: net.CIDRMask(8, 32)}
}

// private12BitBlock returns a private network (RFC1918) 172.16.0.0/12 IPv4 block.
func private12BitBlock() *net.IPNet {
	return &net.IPNet{IP: net.IP{172, 16, 0, 0}, Mask: net.CIDRMask(12, 32)}
}

// private16BitBlock returns a private network (RFC1918) 192.168.0.0/16 IPv4 block.
func private16BitBlock() *net.IPNet {
	return &net.IPNet{IP: net.IP{192, 168, 0, 0}, Mask: net.CIDRMask(16, 32)}
}

// carrierGradeNATBlock returns a Carrier-grade NAT (RFC6598) 100.64.0.0/10 IPv4 block.
func carrierGradeNATBlock() *net.IPNet {
	return &net.IPNet{IP: net.IP{100, 64, 0, 0}, Mask: net.CIDRMask(10, 32)}
}

// allPrivateNetworkBlocks returns a list of all Private network (RFC1918) and Carrier-grade NAT (RFC6598) IPv4 blocks.
func allPrivateNetworkBlocks() []net.IPNet {
	return []net.IPNet{
		*private8BitBlock(),
		*private12BitBlock(),
		*private16BitBlock(),
		*carrierGradeNATBlock(),
	}
}

// toNetworkPolicyPeersWithExceptions returns a list of networkingv1.NetworkPolicyPeers whose ipBlock.cidr points to
// `networks` and whose ipBlock.except points to `except`.
func toNetworkPolicyPeersWithExceptions(networks []net.IPNet, except ...string) ([]networkingv1.NetworkPolicyPeer, error) {
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

// networkPolicyPeersWithExceptions returns a list of networkingv1.NetworkPolicyPeers whose ipBlock.cidr points to
// `networks` and whose ipBlock.except points to `except`.
func networkPolicyPeersWithExceptions(networks []string, except ...string) ([]networkingv1.NetworkPolicyPeer, error) {
	var ipNets []net.IPNet

	for _, n := range networks {
		_, net, err := net.ParseCIDR(string(n))
		if err != nil {
			return nil, err
		}

		ipNets = append(ipNets, *net)
	}

	return toNetworkPolicyPeersWithExceptions(ipNets, except...)
}

func excludeBlock(parentBlock *net.IPNet, cidrs ...string) ([]string, error) {
	var matchedCIDRs []string

	for _, cidr := range cidrs {
		ip, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}
		parentBlockMaskLength, _ := parentBlock.Mask.Size()
		ipNetMaskLength, _ := ipNet.Mask.Size()

		if parentBlock.Contains(ip) && parentBlockMaskLength < ipNetMaskLength {
			matchedCIDRs = append(matchedCIDRs, cidr)
		}
	}

	return matchedCIDRs, nil
}
