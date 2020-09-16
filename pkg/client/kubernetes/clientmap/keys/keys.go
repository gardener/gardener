// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package keys

import (
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/internal"
)

// ForGarden returns a key for retrieving a ClientSet for the Garden cluster.
func ForGarden() clientmap.ClientSetKey {
	return internal.GardenClientSetKey{}
}

// ForSeed returns a key for retrieving a ClientSet for the given Seed cluster.
func ForSeed(seed *v1beta1.Seed) clientmap.ClientSetKey {
	return internal.SeedClientSetKey(seed.Name)
}

// ForSeedWithName returns a key for retrieving a ClientSet for the Seed cluster with the given name.
func ForSeedWithName(name string) clientmap.ClientSetKey {
	return internal.SeedClientSetKey(name)
}

// ForShoot returns a key for retrieving a ClientSet for the given Shoot cluster.
func ForShoot(shoot *v1beta1.Shoot) clientmap.ClientSetKey {
	return internal.ShootClientSetKey{
		Namespace: shoot.Namespace,
		Name:      shoot.Name,
	}
}

// ForShootWithNamespacedName returns a key for retrieving a ClientSet for the Shoot cluster with the given
// namespace and name.
func ForShootWithNamespacedName(namespace, name string) clientmap.ClientSetKey {
	return internal.ShootClientSetKey{
		Namespace: namespace,
		Name:      name,
	}
}

// ForPlant returns a key for retrieving a ClientSet for the given Plant cluster.
func ForPlant(plant *v1beta1.Plant) clientmap.ClientSetKey {
	return internal.PlantClientSetKey{
		Namespace: plant.Namespace,
		Name:      plant.Name,
	}
}

// ForPlantWithNamespacedName returns a key for retrieving a ClientSet for the Plant cluster with the given
// namespace and name.
func ForPlantWithNamespacedName(namespace, name string) clientmap.ClientSetKey {
	return internal.PlantClientSetKey{
		Namespace: namespace,
		Name:      name,
	}
}
