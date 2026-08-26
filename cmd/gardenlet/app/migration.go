// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/features"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

func (g *garden) runMigrations(ctx context.Context, gardenClient client.Client) error {
	if features.DefaultFeatureGate.Enabled(features.RemoveHTTPProxyLegacyPort) {
		if err := VerifyRemoveHTTPProxyLegacyPortMigration(ctx, gardenClient, g.config.SeedConfig.Name); err != nil {
			return fmt.Errorf("failed to verify migration for RemoveHTTPProxyLegacyPort feature gate: %w", err)
		}
	}

	return nil
}

// VerifyRemoveHTTPProxyLegacyPortMigration returns an error if any shoot on this seed has not yet been reconfigured to
// use the unified `http-proxy` port, i.e. if removing the legacy `tls-tunnel` port would break it.
// TODO(jamand): Remove when feature gate RemoveHTTPProxyLegacyPort is removed.
func VerifyRemoveHTTPProxyLegacyPortMigration(ctx context.Context, gardenClient client.Client, seedName string) error {
	// List all (eligible) Shoot resources managed by this seed.
	shootList := &gardencorev1beta1.ShootList{}
	if err := gardenClient.List(ctx, shootList); err != nil {
		return err
	}

	for _, shoot := range shootList.Items {
		if specSeedName, statusSeedName := gardenerutils.GetShootSeedNames(&shoot); gardenerutils.GetResponsibleSeedName(specSeedName, statusSeedName) != seedName {
			continue
		}

		// Skip workerless Shoots (UsesUnifiedHTTPProxyPort is never set for workerless Shoots).
		if helper.IsWorkerless(&shoot) {
			continue
		}

		// Skip if not picked up yet or Creating/Deleting.
		if shoot.Status.LastOperation == nil || ((shoot.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeCreate || shoot.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeDelete) && shoot.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded) {
			continue
		}

		if cond := helper.GetCondition(shoot.Status.Constraints, gardencorev1beta1.ShootUsesUnifiedHTTPProxyPort); cond == nil || cond.Status != gardencorev1beta1.ConditionTrue {
			return errors.New("the `tls-tunnel` port on the istio ingress gateway cannot be removed until the api server proxy and vpn client in all shoots on this seed have been reconfigured to use the unified `http-proxy` port instead, i.e., the `RemoveHTTPProxyLegacyPort` feature gate can only be enabled once all shoots have the `UsesUnifiedHTTPProxyPort` constraint with status `true`")
		}
	}

	return nil
}
