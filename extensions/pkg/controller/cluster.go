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
	// GetOwnerNameAndID reads the owner DNS name and ID from the owner DNSRecord extension resource in the given namespace.
	GetOwnerNameAndID = extensions.GetOwnerNameAndID
	// GenericTokenKubeconfigSecretNameFromCluster reads the generic-token-kubeconfig.secret.gardener.cloud/name annotation
	// and returns its value. If the annotation is not present then it falls back to the deprecated
	// SecretNameGenericTokenKubeconfig.
	GenericTokenKubeconfigSecretNameFromCluster = extensions.GenericTokenKubeconfigSecretNameFromCluster
)
