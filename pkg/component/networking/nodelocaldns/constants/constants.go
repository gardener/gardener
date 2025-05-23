// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	// IPVSAddress is the IPv4 address used by node-local-dns when IPVS is used.
	IPVSAddress = "169.254.20.10"
	// IPVSIPv6Address is the IPv6 address used by node-local-dns when IPVS is used.
	IPVSIPv6Address = "fd30:1319:f1e:230b::1"
	// LabelValue is the value of a label used for the identification of node-local-dns pods.
	LabelValue = "node-local-dns"
)
