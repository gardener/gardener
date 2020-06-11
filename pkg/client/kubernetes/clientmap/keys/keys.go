// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
