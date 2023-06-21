// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shoot

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/care"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/shoot"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/state"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// AddToManager adds all Shoot controllers to the given manager.
func AddToManager(
	ctx context.Context,
	mgr manager.Manager,
	gardenCluster cluster.Cluster,
	seedCluster cluster.Cluster,
	seedClientSet kubernetes.Interface,
	shootClientMap clientmap.ClientMap,
	cfg config.GardenletConfiguration,
	identity *gardencorev1beta1.Gardener,
	gardenClusterIdentity string,
	imageVector imagevector.ImageVector,
) error {
	var responsibleForUnmanagedSeed bool
	if err := gardenCluster.GetAPIReader().Get(ctx, client.ObjectKey{Name: cfg.SeedConfig.Name, Namespace: v1beta1constants.GardenNamespace}, &seedmanagementv1alpha1.ManagedSeed{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed checking whether gardenlet is responsible for a managed seed: %w", err)
		}
		// ManagedSeed was not found, hence gardenlet is responsible for an unmanaged seed.
		responsibleForUnmanagedSeed = true
	}
	shootStateControllerEnabled := responsibleForUnmanagedSeed && pointer.IntDeref(cfg.Controllers.ShootState.ConcurrentSyncs, 0) > 0

	if err := (&shoot.Reconciler{
		SeedClientSet:               seedClientSet,
		ShootClientMap:              shootClientMap,
		Config:                      cfg,
		ImageVector:                 imageVector,
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
		ImageVector:           imageVector,
		Identity:              identity,
		GardenClusterIdentity: gardenClusterIdentity,
		SeedName:              cfg.SeedConfig.Name,
	}).AddToManager(mgr, gardenCluster); err != nil {
		return fmt.Errorf("failed adding care reconciler: %w", err)
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
