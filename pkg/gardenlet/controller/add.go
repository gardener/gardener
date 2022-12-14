// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controller

import (
	"fmt"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/controller/backupbucket"
	"github.com/gardener/gardener/pkg/gardenlet/controller/backupentry"
	"github.com/gardener/gardener/pkg/gardenlet/controller/bastion"
	"github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation"
	"github.com/gardener/gardener/pkg/gardenlet/controller/managedseed"
	"github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy"
	"github.com/gardener/gardener/pkg/gardenlet/controller/seed"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shootstate"
	"github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// AddToManager adds all gardenlet controllers to the given manager.
func AddToManager(
	mgr manager.Manager,
	gardenCluster cluster.Cluster,
	seedCluster cluster.Cluster,
	seedClientSet kubernetes.Interface,
	shootClientMap clientmap.ClientMap,
	cfg *config.GardenletConfiguration,
	gardenNamespace *corev1.Namespace,
	gardenClusterIdentity string,
	identity *gardencorev1beta1.Gardener,
	healthManager healthz.Manager,
) error {
	imageVector, err := imagevector.ReadGlobalImageVectorWithEnvOverride(filepath.Join(charts.Path, "images.yaml"))
	if err != nil {
		return fmt.Errorf("failed reading image vector override: %w", err)
	}

	var componentImageVectors imagevector.ComponentImageVectors
	if path := os.Getenv(imagevector.ComponentOverrideEnv); path != "" {
		componentImageVectors, err = imagevector.ReadComponentOverwriteFile(path)
		if err != nil {
			return fmt.Errorf("failed reading component-specific image vector override: %w", err)
		}
	}

	if err := (&backupbucket.Reconciler{
		Config:   *cfg.Controllers.BackupBucket,
		SeedName: cfg.SeedConfig.Name,
	}).AddToManager(mgr, gardenCluster, seedCluster); err != nil {
		return fmt.Errorf("failed adding BackupBucket controller: %w", err)
	}

	if err := backupentry.AddToManager(mgr, gardenCluster, seedCluster, *cfg); err != nil {
		return fmt.Errorf("failed adding BackupEntry controller: %w", err)
	}

	if err := (&bastion.Reconciler{
		Config: *cfg.Controllers.Bastion,
	}).AddToManager(mgr, gardenCluster, seedCluster); err != nil {
		return fmt.Errorf("failed adding Bastion controller: %w", err)
	}

	if err := controllerinstallation.AddToManager(mgr, gardenCluster, seedCluster, seedClientSet, *cfg, identity, gardenNamespace, gardenClusterIdentity); err != nil {
		return fmt.Errorf("failed adding ControllerInstallation controller: %w", err)
	}

	if err := (&managedseed.Reconciler{
		Config: *cfg.Controllers.ManagedSeed,
	}).AddToManager(mgr, *cfg, gardenCluster, seedCluster, shootClientMap, imageVector); err != nil {
		return fmt.Errorf("failed adding ManagedSeed controller: %w", err)
	}

	if err := (&networkpolicy.Reconciler{
		Config:          *cfg.Controllers.SeedAPIServerNetworkPolicy,
		GardenNamespace: gardenNamespace.Name,
	}).AddToManager(mgr, seedCluster); err != nil {
		return fmt.Errorf("failed adding NetworkPolicy controller: %w", err)
	}

	if err := seed.AddToManager(mgr, gardenCluster, seedCluster, seedClientSet, *cfg, identity, healthManager, imageVector, componentImageVectors); err != nil {
		return fmt.Errorf("failed adding Seed controller: %w", err)
	}

	if err := shoot.AddToManager(mgr, gardenCluster, seedClientSet, shootClientMap, *cfg, identity, gardenClusterIdentity, imageVector); err != nil {
		return fmt.Errorf("failed adding Shoot controller: %w", err)
	}

	if err := shootstate.AddToManager(mgr, gardenCluster, seedCluster, *cfg); err != nil {
		return fmt.Errorf("failed adding ShootState controller: %w", err)
	}

	return nil
}
