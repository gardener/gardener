// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"context"
	"fmt"
	"path/filepath"

	"github.com/go-logr/logr"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	managedseedcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/managedseed"
	shootcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// LegacyControllerFactory starts gardenlet's legacy controllers under leader election of the given manager for
// the purpose of gradually migrating to native controller-runtime controllers.
// Deprecated: this will be replaced by adding native controllers directly to the manager.
// New controllers should be implemented as native controller-runtime controllers right away and should be added to
// the manager directly.
type LegacyControllerFactory struct {
	GardenCluster         cluster.Cluster
	SeedCluster           cluster.Cluster
	SeedClientSet         kubernetes.Interface
	ShootClientMap        clientmap.ClientMap
	Log                   logr.Logger
	Config                *config.GardenletConfiguration
	GardenClusterIdentity string
	Identity              *gardencorev1beta1.Gardener
}

// Start starts all legacy controllers.
func (f *LegacyControllerFactory) Start(ctx context.Context) error {
	log := f.Log.WithName("controller")

	imageVector, err := imagevector.ReadGlobalImageVectorWithEnvOverride(filepath.Join(charts.Path, "images.yaml"))
	if err != nil {
		return fmt.Errorf("failed reading image vector override: %w", err)
	}

	managedSeedController, err := managedseedcontroller.NewManagedSeedController(ctx, log, f.GardenCluster, f.SeedCluster, f.ShootClientMap, f.Config, imageVector)
	if err != nil {
		return fmt.Errorf("failed initializing ManagedSeed controller: %w", err)
	}

	shootController, err := shootcontroller.NewShootController(ctx, log, f.GardenCluster, f.SeedClientSet, f.ShootClientMap, f.Config, f.Identity, f.GardenClusterIdentity, imageVector, clock.RealClock{})
	if err != nil {
		return fmt.Errorf("failed initializing Shoot controller: %w", err)
	}

	controllerCtx, cancel := context.WithCancel(ctx)

	// run controllers
	go managedSeedController.Run(controllerCtx, *f.Config.Controllers.ManagedSeed.ConcurrentSyncs)
	go shootController.Run(controllerCtx, *f.Config.Controllers.Shoot.ConcurrentSyncs, *f.Config.Controllers.ShootCare.ConcurrentSyncs)

	log.Info("gardenlet initialized")

	// block until shutting down
	<-ctx.Done()
	cancel()
	return nil
}
