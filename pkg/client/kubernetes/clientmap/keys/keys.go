// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package keys

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
)

// ForGarden returns a key for retrieving a ClientSet for the given Shoot cluster.
func ForGarden(garden *operatorv1alpha1.Garden) clientmap.ClientSetKey {
	return clientmap.GardenClientSetKey{
		Name: garden.Name,
	}
}

// ForShoot returns a key for retrieving a ClientSet for the given Shoot cluster.
func ForShoot(shoot *gardencorev1beta1.Shoot) clientmap.ClientSetKey {
	return clientmap.ShootClientSetKey{
		Namespace: shoot.Namespace,
		Name:      shoot.Name,
	}
}

// ForShootWithNamespacedName returns a key for retrieving a ClientSet for the Shoot cluster with the given
// namespace and name.
func ForShootWithNamespacedName(namespace, name string) clientmap.ClientSetKey {
	return clientmap.ShootClientSetKey{
		Namespace: namespace,
		Name:      name,
	}
}
