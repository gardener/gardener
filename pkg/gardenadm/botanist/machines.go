// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
)

// addressTypePreference when picking a node/machine address. Higher value means higher priority.
// Unknown address types have the lowest priority (0).
var addressTypePreference = map[corev1.NodeAddressType]int{
	// internal names have priority: they are reachable from within the cluster network even when nodes have no
	// externally routable addresses (and for SSH we jump via a bastion host)
	corev1.NodeInternalIP:  5,
	corev1.NodeInternalDNS: 4,
	corev1.NodeExternalIP:  3,
	corev1.NodeExternalDNS: 2,
	// this should be returned by all providers, and is actually locally resolvable in many infrastructures
	corev1.NodeHostName: 1,
}

// PreferredNodeAddress returns the preferred address of the given node/machine addresses based on
// addressTypePreference (prefer internal and IP to external and DNS). Returns an error if no addresses are given.
func PreferredNodeAddress(addresses []corev1.NodeAddress) (string, error) {
	if len(addresses) == 0 {
		return "", fmt.Errorf("no addresses found in status")
	}

	address := slices.MaxFunc(addresses, func(a, b corev1.NodeAddress) int {
		return addressTypePreference[a.Type] - addressTypePreference[b.Type]
	})

	return address.Address, nil
}
