// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package net

import (
	"net/netip"
)

// GetBitLen returns the bit length of the given IP address.
func GetBitLen(address string) (int, error) {
	ip, err := netip.ParseAddr(address)
	if err != nil {
		return 0, err
	}
	return ip.BitLen(), nil
}
