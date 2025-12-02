// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/go-logr/logr"

	"github.com/gardener/gardener/cmd/internal/migration"
	"github.com/gardener/gardener/pkg/features"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger) error {
	if features.DefaultFeatureGate.Enabled(features.VPAInPlaceUpdates) {
		if err := migration.MigrateEmptyVPAPatch(ctx, g.mgr, log); err != nil {
			return err
		}
	}
	return nil
}
