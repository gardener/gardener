// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/cmd/internal/migration"
	"github.com/gardener/gardener/pkg/features"
)

func runMigrations(ctx context.Context, mgr manager.Manager, log logr.Logger) error {
	if features.DefaultFeatureGate.Enabled(features.VPAInPlaceUpdates) {
		if err := migration.MigrateVPAEmptyPatch(ctx, mgr.GetClient(), log); err != nil {
			return err
		}
	} else {
		if err := migration.MigrateVPAUpdateModeToRecreate(ctx, mgr.GetClient(), log); err != nil {
			return err
		}
	}
	return nil
}
