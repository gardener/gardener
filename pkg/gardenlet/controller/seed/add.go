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

package seed

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/controller/seed/care"
	"github.com/gardener/gardener/pkg/gardenlet/controller/seed/lease"
	"github.com/gardener/gardener/pkg/gardenlet/controller/seed/seed"
	"github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// AddToManager adds all Seed controllers to the given manager.
func AddToManager(
	mgr manager.Manager,
	gardenCluster cluster.Cluster,
	seedCluster cluster.Cluster,
	seedClientSet kubernetes.Interface,
	cfg config.GardenletConfiguration,
	identity *gardencorev1beta1.Gardener,
	healthManager healthz.Manager,
	imageVector imagevector.ImageVector,
	componentImageVectors imagevector.ComponentImageVectors,
) error {
	if err := (&care.Reconciler{
		Config:   *cfg.Controllers.SeedCare,
		SeedName: cfg.SeedConfig.Name,
	}).AddToManager(mgr, gardenCluster, seedCluster); err != nil {
		return fmt.Errorf("failed adding care reconciler: %w", err)
	}

	if err := (&lease.Reconciler{
		SeedRESTClient: seedClientSet.RESTClient(),
		Config:         *cfg.Controllers.Seed,
		HealthManager:  healthManager,
		SeedName:       cfg.SeedConfig.Name,
	}).AddToManager(mgr, gardenCluster); err != nil {
		return fmt.Errorf("failed adding lease reconciler: %w", err)
	}

	if err := (&seed.Reconciler{
		SeedClientSet:         seedClientSet,
		Config:                cfg,
		Identity:              identity,
		ImageVector:           imageVector,
		ComponentImageVectors: componentImageVectors,
	}).AddToManager(mgr, gardenCluster); err != nil {
		return fmt.Errorf("failed adding main reconciler: %w", err)
	}

	return nil
}
