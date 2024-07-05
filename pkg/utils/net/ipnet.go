// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package net

import (
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
