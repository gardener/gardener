// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	// IPVSAddress is the IPv4 address used by node-local-dns when IPVS is used.
	IPVSAddress = "169.254.20.10"
	// LabelValue is the value of a label used for the identification of node-local-dns pods.
	LabelValue = "node-local-dns"
)
