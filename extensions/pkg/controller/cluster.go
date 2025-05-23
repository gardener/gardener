// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import "github.com/gardener/gardener/pkg/extensions"

// Cluster contains the decoded resources of Gardener's extension Cluster resource.
type Cluster = extensions.Cluster

var (
	// GetCluster tries to read Gardener's Cluster extension resource in the given namespace.
	GetCluster = extensions.GetCluster
	// CloudProfileFromCluster returns the CloudProfile resource inside the Cluster resource.
	CloudProfileFromCluster = extensions.CloudProfileFromCluster
	// SeedFromCluster returns the Seed resource inside the Cluster resource.
	SeedFromCluster = extensions.SeedFromCluster
	// ShootFromCluster returns the Shoot resource inside the Cluster resource.
	ShootFromCluster = extensions.ShootFromCluster
	// GetShoot tries to read Gardener's Cluster extension resource in the given namespace and return the embedded Shoot resource.
	GetShoot = extensions.GetShoot
	// GenericTokenKubeconfigSecretNameFromCluster reads the generic-token-kubeconfig.secret.gardener.cloud/name annotation
	// and returns its value. If the annotation is not present then it falls back to the deprecated
	// SecretNameGenericTokenKubeconfig.
	GenericTokenKubeconfigSecretNameFromCluster = extensions.GenericTokenKubeconfigSecretNameFromCluster
)
