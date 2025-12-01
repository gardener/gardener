// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/gardener/gardener/cmd/internal/migration"
	"github.com/gardener/gardener/pkg/features"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func runMigrations(ctx context.Context, mgr manager.Manager, log logr.Logger) error {
	if features.DefaultFeatureGate.Enabled(features.VPAInPlaceUpdates) {
		if err := migration.MigrateEmptyVPAPatch(ctx, mgr, log); err != nil {
			return err
		}
	}
	return nil
}
