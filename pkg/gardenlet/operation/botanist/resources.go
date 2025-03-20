// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const managedResourceName = "referenced-resources"

// DeployReferencedResources reads all referenced resources from the Garden cluster and writes a managed resource to the Seed cluster.
func (b *Botanist) DeployReferencedResources(ctx context.Context) error {
	unstructuredObjs, err := gardenerutils.PrepareReferencedResourcesForSeedCopy(ctx, b.GardenClient, b.Shoot.GetInfo().Spec.Resources, b.Shoot.GetInfo().Namespace, b.Shoot.ControlPlaneNamespace)
	if err != nil {
		return err
	}

	// Create managed resource from the slice of unstructured objects
	return managedresources.CreateFromUnstructured(ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, managedResourceName,
		false, v1beta1constants.SeedResourceManagerClass, unstructuredObjs, false, nil)
}

// DestroyReferencedResources deletes the managed resource containing referenced resources from the Seed cluster.
func (b *Botanist) DestroyReferencedResources(ctx context.Context) error {
	return client.IgnoreNotFound(managedresources.Delete(ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, managedResourceName, false))
}
