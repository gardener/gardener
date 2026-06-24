// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	"github.com/gardener/gardener/cmd/internal/migration"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger) error {
	seedClient := g.mgr.GetClient()

	// The below migration is preserved in order to cover the migration when upgrading from
	// VPAInPlaceUpdates feature gate disabled to VPAInPlaceUpdates enabled unconditionally.
	//
	// TODO(ialidzhikov): Clean up the migration below when cleaning up the VPAInPlaceUpdates feature gate.
	if err := migration.MigrateVPAEmptyPatch(ctx, seedClient, log); err != nil {
		return fmt.Errorf("failed to migrate VerticalPodAutoscaler with 'MigrateVPAEmptyPatch' migration: %w", err)
	}

	return nil
}
