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

package shoot

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/care"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// AddToManager adds all Shoot controllers to the given manager.
func AddToManager(
	mgr manager.Manager,
	gardenCluster cluster.Cluster,
	seedClientSet kubernetes.Interface,
	shootClientMap clientmap.ClientMap,
	cfg config.GardenletConfiguration,
	identity *gardencorev1beta1.Gardener,
	gardenClusterIdentity string,
	imageVector imagevector.ImageVector,
) error {
	if err := (&shoot.Reconciler{
		SeedClientSet:         seedClientSet,
		ShootClientMap:        shootClientMap,
		Config:                cfg,
		ImageVector:           imageVector,
		Identity:              identity,
		GardenClusterIdentity: gardenClusterIdentity,
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

	return nil
}
