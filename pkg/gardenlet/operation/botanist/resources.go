// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

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
		return fmt.Errorf("failed to prepare referenced resources for seed copy: %w", err)
	}

	// Create managed resource from the slice of unstructured objects
	if err := managedresources.CreateFromUnstructured(
		ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, managedResourceName,
		false, v1beta1constants.SeedResourceManagerClass, unstructuredObjs, false, nil,
	); err != nil {
		return fmt.Errorf("failed to create managed resource for referenced resources: %w", err)
	}

	// Reconcile secrets for referenced WorkloadIdentities
	if err := gardenerutils.ReconcileWorkloadIdentityReferencedResources(
		ctx, b.GardenClient, b.SeedClientSet.Client(), b.Shoot.GetInfo().Spec.Resources,
		b.Shoot.GetInfo().Namespace, b.Shoot.ControlPlaneNamespace, b.Shoot.GetInfo(),
	); err != nil {
		return fmt.Errorf("failed to reconcile referenced workload identities: %w", err)
	}

	return nil
}

// DestroyReferencedResources deletes the managed resource containing referenced resources from the Seed cluster.
func (b *Botanist) DestroyReferencedResources(ctx context.Context) error {
	if err := gardenerutils.DestroyWorkloadIdentityReferencedResources(ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace); err != nil {
		return fmt.Errorf("failed to destroy referenced workload identities: %w", err)
	}

	if err := client.IgnoreNotFound(managedresources.Delete(ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, managedResourceName, false)); err != nil {
		return fmt.Errorf("failed to delete managed resource for referenced resources: %w", err)
	}

	return nil
}
