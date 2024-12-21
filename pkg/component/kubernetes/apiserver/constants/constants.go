// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	// Port is the port exposed by the kube-apiserver.
	Port = 443
	// RequestHeaderGroupHeaders is the header key for the group headers.
	RequestHeaderGroupHeaders = "X-Remote-Group"
	// RequestHeaderUserNameHeaders is the header key for the username headers.
	RequestHeaderUserNameHeaders = "X-Remote-User"
)

// ServiceName returns the service name with the given prefix.
func ServiceName(prefix string) string {
	return prefix + "kube-apiserver"
}
