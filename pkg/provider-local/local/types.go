// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Name is the name of the local provider.
	Name = "provider-local"
	// Type is the type of resources managed by the local actuators.
	Type = "local"

	// GardenRoleInfra is the value of the GardenRole key indicating type 'infra'.
	GardenRoleInfra = "infra"

	// FieldOwner is a constant for the owner name in `.metadata.managedFields`.
	FieldOwner = client.FieldOwner("gardener-extension-provider-local")

	// CloudProviderConfigName is the name of the configmap containing the cloud provider config.
	CloudProviderConfigName = "cloud-provider-config"

	// CloudControllerManagerName is a constant for the name of the cloud-controller-manager deployed by the controlplane controller.
	CloudControllerManagerName = "cloud-controller-manager"
)

var (
	// NodeResourceCPU is the resource that will be used for advertising the node's CPU capacity.
	NodeResourceCPU = resource.MustParse("100")
	// NodeResourceMemory is the resource that will be used for advertising the node's memory capacity.
	NodeResourceMemory = resource.MustParse("100Gi")
)
