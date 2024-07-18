// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package net

import (
	"fmt"
	"net"
	"strings"
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
