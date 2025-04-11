// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/care"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/shoot"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/state"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/status"
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
) error {
	var responsibleForUnmanagedSeed bool
	if err := gardenCluster.GetAPIReader().Get(ctx, client.ObjectKey{Name: cfg.SeedConfig.Name, Namespace: v1beta1constants.GardenNamespace}, &seedmanagementv1alpha1.ManagedSeed{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed checking whether gardenlet is responsible for a managed seed: %w", err)
		}
		// ManagedSeed was not found, hence gardenlet is responsible for an unmanaged seed.
		responsibleForUnmanagedSeed = true
	}
	shootStateControllerEnabled := responsibleForUnmanagedSeed && ptr.Deref(cfg.Controllers.ShootState.ConcurrentSyncs, 0) > 0

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

	if err := (&care.Reconciler{
		SeedClientSet:         seedClientSet,
		ShootClientMap:        shootClientMap,
		Config:                cfg,
		Identity:              identity,
		GardenClusterIdentity: gardenClusterIdentity,
		SeedName:              cfg.SeedConfig.Name,
	}).AddToManager(mgr, gardenCluster); err != nil {
		return fmt.Errorf("failed adding care reconciler: %w", err)
	}

	if err := (&status.Reconciler{
		Config:   *cfg.Controllers.ShootStatus,
		SeedName: cfg.SeedConfig.Name,
	}).AddToManager(mgr, gardenCluster, seedCluster); err != nil {
		return fmt.Errorf("failed adding status reconciler: %w", err)
	}

	// If gardenlet is responsible for an unmanaged seed we want to add the state reconciler which performs periodic
	// backups of shoot states (see GEP-22).
	if shootStateControllerEnabled {
		mgr.GetLogger().Info("Adding shoot state reconciler since gardenlet is responsible for an unmanaged seed")

		if err := (&state.Reconciler{
			Config:   *cfg.Controllers.ShootState,
			SeedName: cfg.SeedConfig.Name,
		}).AddToManager(mgr, gardenCluster, seedCluster); err != nil {
			return fmt.Errorf("failed adding state reconciler: %w", err)
		}
	}

	return nil
}
