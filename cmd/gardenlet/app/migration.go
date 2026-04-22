// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/gardener/gardener/cmd/internal/migration"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/gardener/shootstate"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger, gardenCluster cluster.Cluster) error {
	if features.DefaultFeatureGate.Enabled(features.VPAInPlaceUpdates) {
		if err := migration.MigrateVPAEmptyPatch(ctx, g.mgr.GetClient(), log); err != nil {
			return fmt.Errorf("failed to migrate VerticalPodAutoscaler with 'MigrateVPAEmptyPatch' migration: %w", err)
		}
	} else {
		if err := migration.MigrateVPAUpdateModeToRecreate(ctx, g.mgr.GetClient(), log); err != nil {
			return fmt.Errorf("failed to migrate VerticalPodAutoscaler with 'MigrateVPAUpdateModeToRecreate' migration: %w", err)
		}
	}

	if g.selfHostedShootInfo == nil {
		if err := g.removeObsoleteShootStates(ctx, log, gardenCluster.GetAPIReader(), gardenCluster.GetClient()); err != nil {
			return fmt.Errorf("failed to remove obsolete ShootStates: %w", err)
		}
	}

	return nil
}

// A bug in v1.140 caused unintended ShootState creations for Shoots running on ManagedSeeds. This function is
// making gardenlet cleaning them up automatically (when responsible for a managed seed).
// TODO(tobschli): Remove this migration after v1.143 has been released.
func (g *garden) removeObsoleteShootStates(ctx context.Context, log logr.Logger, apiReader client.Reader, c client.Client) error {
	if err := apiReader.Get(ctx, client.ObjectKey{Name: g.config.SeedConfig.Name, Namespace: v1beta1constants.GardenNamespace}, &seedmanagementv1alpha1.ManagedSeed{}); err != nil {
		// Forbidden is treated like NotFound because the SeedAuthorizer only grants access to ManagedSeeds
		// related to this seed via the resource graph. For unmanaged seeds, no ManagedSeed exists and no graph
		// edge is present, so the authorizer returns Forbidden.
		if !apierrors.IsNotFound(err) && !apierrors.IsForbidden(err) {
			return fmt.Errorf("failed checking whether gardenlet is responsible for a managed seed: %w", err)
		}
		// ManagedSeed was not found, hence gardenlet is responsible for an unmanaged seed.
		log.Info("gardenlet is responsible for unmanaged seed, skip unintended ShootState cleanup")
		return nil
	}

	log.Info("gardenlet is responsible for a managed seed, check if unintendedly created ShootStates (due to a bug in gardenlet@v1.140.{0,1}) have to be cleaned up")

	shootList := &gardencorev1beta1.ShootList{}
	if err := c.List(ctx, shootList); err != nil {
		return fmt.Errorf("failed listing Shoots: %w", err)
	}

	var (
		shootInMigrationOrRestore = func(status gardencorev1beta1.ShootStatus) bool {
			return status.LastOperation != nil &&
				(status.LastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate ||
					(status.LastOperation.Type == gardencorev1beta1.LastOperationTypeRestore &&
						status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded))
		}
		tasks []flow.TaskFn
	)

	for _, shoot := range shootList.Items {
		if shootInMigrationOrRestore(shoot.Status) {
			log.Info("Skipping Shoot in migration or restore for obsolete ShootState cleanup", "shoot", client.ObjectKeyFromObject(&shoot))
			continue
		}

		tasks = append(tasks, func(ctx context.Context) error {
			log.Info("Deleting unintendedly created ShootState (if it exists)", "shootState", client.ObjectKeyFromObject(&shoot))
			return shootstate.Delete(ctx, c, &shoot)
		})
	}

	return flow.Parallel(tasks...)(ctx)
}
