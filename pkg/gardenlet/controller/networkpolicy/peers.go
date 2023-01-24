// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
)

// private8BitBlock returns a private network (RFC1918) 10.0.0.0/8 IPv4 block
func private8BitBlock() *net.IPNet {
	return &net.IPNet{IP: net.IP{10, 0, 0, 0}, Mask: net.CIDRMask(8, 32)}
}

// private12BitBlock returns a private network (RFC1918) 172.16.0.0/12 IPv4 block
func private12BitBlock() *net.IPNet {
	return &net.IPNet{IP: net.IP{172, 16, 0, 0}, Mask: net.CIDRMask(12, 32)}
}

// private16BitBlock returns a private network (RFC1918) 192.168.0.0/16 IPv4 block
func private16BitBlock() *net.IPNet {
	return &net.IPNet{IP: net.IP{192, 168, 0, 0}, Mask: net.CIDRMask(16, 32)}
}

// carrierGradeNATBlock returns a Carrier-grade NAT (RFC6598) 100.64.0.0/10 IPv4 block
func carrierGradeNATBlock() *net.IPNet {
	return &net.IPNet{IP: net.IP{100, 64, 0, 0}, Mask: net.CIDRMask(10, 32)}
}
