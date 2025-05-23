// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package net

import (
	"fmt"
	"net"
	"strings"
)

const (
	// IPv4Family represents the IPv4 IP family.
	IPv4Family = "ipv4"
	// IPv6Family represents the IPv6 IP family.
	IPv6Family = "ipv6"
)

// JoinByComma concatenates the CIDRs of the given networks to create a single string with comma as separator.
func JoinByComma(cidrs []net.IPNet) string {
	return Join(cidrs, ",")
}

// Join concatenates the CIDRs of the given networks to create a single string with a custom separator character.
func Join(cidrs []net.IPNet, sep string) string {
	result := ""
	for _, cidr := range cidrs {
		result += cidr.String() + sep
	}
	return strings.TrimSuffix(result, sep)
}

// CheckDualStackForKubeComponents checks if the given list of CIDRs does not include more than one element of the same IP family.
func CheckDualStackForKubeComponents(cidrs []net.IPNet, networkType string) error {
	if len(cidrs) > 2 {
		return fmt.Errorf("%s network CIDRs must not contain more than two elements: '%s'", networkType, cidrs)
	}

	if len(cidrs) == 2 {
		if dualStack, err := dualStack(cidrs); err != nil {
			return fmt.Errorf("invalid %s network CIDRs ('%s'): %w", networkType, cidrs, err)
		} else if !dualStack {
			return fmt.Errorf("%s network CIDRs must be of different IP family: '%s'", networkType, cidrs)
		}
	}

	return nil
}

func dualStack(cidrs []net.IPNet) (bool, error) {
	v4 := false
	v6 := false
	for _, cidr := range cidrs {
		switch true {
		case cidr.IP.To4() != nil:
			v4 = true
		case cidr.IP.To16() != nil:
			v6 = true
		default:
			return false, fmt.Errorf("invalid CIDR: %v", cidr.String())
		}
	}
	return v4 && v6, nil
}

// GetByIPFamily returns a list of CIDRs that belong to the given IP family.
func GetByIPFamily(cidrs []net.IPNet, ipFamily string) []net.IPNet {
	var result []net.IPNet
	for _, nw := range cidrs {
		switch ipFamily {
		case IPv4Family:
			if nw.IP.To4() != nil && len(nw.IP) == net.IPv4len {
				result = append(result, nw)
			}
		case IPv6Family:
			if nw.IP.To16() != nil && len(nw.IP) == net.IPv6len {
				result = append(result, nw)
			}
		}
	}
	return result
}
