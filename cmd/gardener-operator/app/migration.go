// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/cmd/internal/migration"
	"github.com/gardener/gardener/pkg/features"
)

func runMigrations(ctx context.Context, c client.Client, log logr.Logger) error {
	if features.DefaultFeatureGate.Enabled(features.VPAInPlaceUpdates) {
		if err := migration.MigrateVPAEmptyPatch(ctx, c, log); err != nil {
			return fmt.Errorf("failed to migrate VerticalPodAutoscaler with 'MigrateVPAEmptyPatch' migration: %w", err)
		}
	} else {
		if err := migration.MigrateVPAUpdateModeToRecreate(ctx, c, log); err != nil {
			return fmt.Errorf("failed to migrate VerticalPodAutoscaler with 'MigrateVPAUpdateModeToRecreate' migration: %w", err)
		}
	}
	return nil
}
