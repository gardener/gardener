// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/care"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/lease"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/shoot"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/state"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/status"
	"github.com/gardener/gardener/pkg/healthz"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	gardenletutils "github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
)

// AddToManager adds all Shoot controllers to the given manager.
func AddToManager(
	ctx context.Context,
	mgr manager.Manager,
	gardenCluster cluster.Cluster,
	seedCluster cluster.Cluster,
	seedClientSet kubernetes.Interface,
	shootClientMap clientmap.ClientMap,
	cfg gardenletconfigv1alpha1.GardenletConfiguration,
	identity *gardencorev1beta1.Gardener,
	gardenClusterIdentity string,
	healthManager healthz.Manager,
	seedName string,
) error {
	// The ShootState reconciler is only enabled when:
	// (a) the gardenlet is responsible for a self-hosted shoot (we always want ShootStates for such clusters in the
	//     garden cluster), or
	// (b) the gardenlet is responsible for an unmanaged seed and the controller is enabled in its component config (see
	//     GEP-0022).
	shootStateControllerEnabled := true
	if !gardenletutils.IsResponsibleForSelfHostedShoot() {
		responsibleForManagedSeed, err := gardenerutils.ClusterIsManagedByManagedSeed(ctx, gardenCluster.GetAPIReader(), cfg.SeedConfig.Name)
		if err != nil {
			return fmt.Errorf("failed checking whether gardenlet is responsible for a managed seed: %w", err)
		}
		shootStateControllerEnabled = !responsibleForManagedSeed && ptr.Deref(cfg.Controllers.ShootState.ConcurrentSyncs, 0) > 0
	}

	// TODO(rfranzke): Enable this reconciler when the work on GEP-0028 progresses.
	if !gardenletutils.IsResponsibleForSelfHostedShoot() {
		if err := (&shoot.Reconciler{
			SeedClientSet:               seedClientSet,
			ShootClientMap:              shootClientMap,
			Config:                      cfg,
			Identity:                    identity,
			GardenClusterIdentity:       gardenClusterIdentity,
			ShootStateControllerEnabled: shootStateControllerEnabled,
		}).AddToManager(mgr, gardenCluster); err != nil {
			return fmt.Errorf("failed adding main reconciler: %w", err)
		}
	}

	if err := (&care.Reconciler{
		SeedClientSet:         seedClientSet,
		ShootClientMap:        shootClientMap,
		Config:                cfg,
		Identity:              identity,
		GardenClusterIdentity: gardenClusterIdentity,
		SeedName:              seedName,
	}).AddToManager(mgr, gardenCluster); err != nil {
		return fmt.Errorf("failed adding care reconciler: %w", err)
	}

	if err := (&status.Reconciler{
		Config:   *cfg.Controllers.ShootStatus,
		SeedName: seedName,
	}).AddToManager(mgr, gardenCluster, seedCluster); err != nil {
		return fmt.Errorf("failed adding status reconciler: %w", err)
	}

	if shootStateControllerEnabled {
		mgr.GetLogger().Info("Adding ShootState since gardenlet is responsible for a self-hosted shoot or for an unmanaged seed")

		if err := (&state.Reconciler{
			Config:   *cfg.Controllers.ShootState,
			SeedName: seedName,
		}).AddToManager(mgr, gardenCluster, seedCluster); err != nil {
			return fmt.Errorf("failed adding state reconciler: %w", err)
		}
	}

	if gardenletutils.IsResponsibleForSelfHostedShoot() {
		if err := lease.AddToManager(mgr, gardenCluster, seedClientSet.RESTClient(), healthManager, nil); err != nil {
			return fmt.Errorf("failed adding lease reconciler: %w", err)
		}
	}

	return nil
}
