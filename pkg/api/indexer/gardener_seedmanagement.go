// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package indexer

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
)

// ManagedSeedShootNameIndexerFunc extracts the .spec.shoot.name field of a ManagedSeed.
func ManagedSeedShootNameIndexerFunc(obj client.Object) []string {
	managedSeed, ok := obj.(*seedmanagementv1alpha1.ManagedSeed)
	if !ok {
		return []string{""}
	}
	if managedSeed.Spec.Shoot == nil {
		return []string{""}
	}
	return []string{managedSeed.Spec.Shoot.Name}
}

// AddManagedSeedShootName adds an index for seedmanagement.ManagedSeedShootName to the given indexer.
func AddManagedSeedShootName(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &seedmanagementv1alpha1.ManagedSeed{}, seedmanagement.ManagedSeedShootName, ManagedSeedShootNameIndexerFunc); err != nil {
		return fmt.Errorf("failed to add indexer for %s to ManagedSeed Informer: %w", seedmanagement.ManagedSeedShootName, err)
	}
	return nil
}
