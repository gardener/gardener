// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	// LabelKey is the key of a label used for the identification of CoreDNS pods.
	LabelKey = "k8s-app"
	// LabelValue is the value of a label used for the identification of CoreDNS pods (it's 'kube-dns' for legacy
	// reasons).
	LabelValue = "kube-dns"
	// PortServiceServer is the service port used for the DNS server.
	PortServiceServer = 53
	// PortServer is the target port used for the DNS server.
	PortServer = 8053
)
