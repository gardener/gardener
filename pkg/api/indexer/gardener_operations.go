// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package indexer

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/operations"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
)

// BastionShootNameIndexerFunc extracts the .spec.shootRef.name field of a Bastion.
func BastionShootNameIndexerFunc(obj client.Object) []string {
	bastion, ok := obj.(*operationsv1alpha1.Bastion)
	if !ok {
		return []string{""}
	}
	return []string{bastion.Spec.ShootRef.Name}
}

// AddBastionShootName adds an index for operations.BastionShootName to the given indexer.
func AddBastionShootName(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &operationsv1alpha1.Bastion{}, operations.BastionShootName, BastionShootNameIndexerFunc); err != nil {
		return fmt.Errorf("failed to add indexer for %s to Bastion Informer: %w", operations.BastionShootName, err)
	}
	return nil
}
