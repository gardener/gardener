// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ShootClient is an interface to be used to receive a shoot client.
type ShootClient interface {
	// InjectShootClient injects the shoot client
	InjectShootClient(client.Client)
}

// SeedClient is an interface to be used to receive a seed client.
type SeedClient interface {
	// InjectSeedClient injects the seed client
	InjectSeedClient(client.Client)
}

// ShootClientInto will set the shoot client on i if i implements ShootClient.
func ShootClientInto(client client.Client, i any) bool {
	if s, ok := i.(ShootClient); ok {
		s.InjectShootClient(client)
		return true
	}
	return false
}

// SeedClientInto will set the seed client on i if i implements SeedClient.
func SeedClientInto(client client.Client, i any) bool {
	if s, ok := i.(SeedClient); ok {
		s.InjectSeedClient(client)
		return true
	}
	return false
}
