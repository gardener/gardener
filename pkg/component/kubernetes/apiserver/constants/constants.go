// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	// Port is the port exposed by the kube-apiserver.
	Port = 443
	// RequestHeaderGroup is the header key for the group headers.
	RequestHeaderGroup = "X-Remote-Group"
	// RequestHeaderUserName is the header key for the username headers.
	RequestHeaderUserName = "X-Remote-User"
	// RequestHeaderPrefixExtra is the prefix headers keys for extra headers.
	RequestHeaderPrefixExtra = "X-Remote-Extra-"
)

// ServiceName returns the service name with the given prefix.
func ServiceName(prefix string) string {
	return prefix + "kube-apiserver"
}
