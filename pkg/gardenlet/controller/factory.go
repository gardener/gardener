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
	"os"
	"path/filepath"
	"time"

	"github.com/gardener/gardener/charts"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	backupbucketcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/backupbucket"
	backupentrycontroller "github.com/gardener/gardener/pkg/gardenlet/controller/backupentry"
	bastioncontroller "github.com/gardener/gardener/pkg/gardenlet/controller/bastion"
	controllerinstallationcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation"
	extensionscontroller "github.com/gardener/gardener/pkg/gardenlet/controller/extensions"
	managedseedcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/managedseed"
	networkpolicycontroller "github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy"
	seedcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/seed"
	shootcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/shoot"
	shootsecretcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/shootsecret"
	"github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
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
	HealthManager         healthz.Manager
	GardenClusterIdentity string
	GardenNamespace       *corev1.Namespace
}

// Start starts all legacy controllers.
func (f *LegacyControllerFactory) Start(ctx context.Context) error {
	log := f.Log.WithName("controller")

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

	identity, err := determineIdentity()
	if err != nil {
		return err
	}

	backupBucketController, err := backupbucketcontroller.NewBackupBucketController(ctx, log, f.GardenCluster, f.SeedCluster, f.Config)
	if err != nil {
		return fmt.Errorf("failed initializing BackupBucket controller: %w", err)
	}

	backupEntryController, err := backupentrycontroller.NewBackupEntryController(ctx, log, f.GardenCluster, f.SeedCluster, f.Config)
	if err != nil {
		return fmt.Errorf("failed initializing BackupEntry controller: %w", err)
	}

	bastionController, err := bastioncontroller.NewBastionController(ctx, log, f.GardenCluster, f.SeedCluster, f.Config)
	if err != nil {
		return fmt.Errorf("failed initializing Bastion controller: %w", err)
	}

	controllerInstallationController, err := controllerinstallationcontroller.NewController(ctx, log, f.GardenCluster, f.SeedClientSet, f.Config, identity, f.GardenNamespace, f.GardenClusterIdentity)
	if err != nil {
		return fmt.Errorf("failed initializing ControllerInstallation controller: %w", err)
	}

	extensionsController := extensionscontroller.NewController(log, f.GardenCluster, f.SeedCluster, f.Config.SeedConfig.Name)

	managedSeedController, err := managedseedcontroller.NewManagedSeedController(ctx, log, f.GardenCluster, f.SeedCluster, f.ShootClientMap, f.Config, imageVector)
	if err != nil {
		return fmt.Errorf("failed initializing ManagedSeed controller: %w", err)
	}

	networkPolicyController, err := networkpolicycontroller.NewController(ctx, log, f.SeedCluster, f.Config.SeedConfig.Name)
	if err != nil {
		return fmt.Errorf("failed initializing NetworkPolicy controller: %w", err)
	}

	secretController, err := shootsecretcontroller.NewController(ctx, log, f.GardenCluster, f.SeedCluster)
	if err != nil {
		return fmt.Errorf("failed initializing Secret controller: %w", err)
	}

	seedController, err := seedcontroller.NewSeedController(ctx, log, f.GardenCluster, f.SeedClientSet, f.HealthManager, imageVector, componentImageVectors, identity, f.Config)
	if err != nil {
		return fmt.Errorf("failed initializing Seed controller: %w", err)
	}

	shootController, err := shootcontroller.NewShootController(ctx, log, f.GardenCluster, f.SeedClientSet, f.ShootClientMap, f.Config, identity, f.GardenClusterIdentity, imageVector)
	if err != nil {
		return fmt.Errorf("failed initializing Shoot controller: %w", err)
	}

	controllerCtx, cancel := context.WithCancel(ctx)

	// run controllers
	go backupBucketController.Run(controllerCtx, *f.Config.Controllers.BackupBucket.ConcurrentSyncs)
	go backupEntryController.Run(controllerCtx, *f.Config.Controllers.BackupEntry.ConcurrentSyncs, *f.Config.Controllers.BackupEntryMigration.ConcurrentSyncs)
	go bastionController.Run(controllerCtx, *f.Config.Controllers.Bastion.ConcurrentSyncs)
	go controllerInstallationController.Run(controllerCtx)
	go managedSeedController.Run(controllerCtx, *f.Config.Controllers.ManagedSeed.ConcurrentSyncs)
	go networkPolicyController.Run(controllerCtx, *f.Config.Controllers.SeedAPIServerNetworkPolicy.ConcurrentSyncs)
	go secretController.Run(controllerCtx, *f.Config.Controllers.ShootSecret.ConcurrentSyncs)
	go seedController.Run(controllerCtx, *f.Config.Controllers.Seed.ConcurrentSyncs)
	go shootController.Run(controllerCtx, *f.Config.Controllers.Shoot.ConcurrentSyncs, *f.Config.Controllers.ShootCare.ConcurrentSyncs, *f.Config.Controllers.ShootMigration.ConcurrentSyncs)

	// TODO(timebertt): This can be removed once we have refactored the extensions controller to a native controller-runtime controller
	//   With https://github.com/kubernetes-sigs/controller-runtime/pull/1678 source.Kind already retries getting
	//   an informer on NoKindMatch (just make sure, /readyz fails until we have an informer)
	if err := retry.Until(ctx, 10*time.Second, func(ctx context.Context) (bool, error) {
		if err := extensionsController.Initialize(ctx, f.SeedCluster); err != nil {
			// A NoMatchError most probably indicates that the necessary CRDs haven't been deployed to the affected seed cluster yet.
			// This can either be the case if the seed cluster is new or if a new extension CRD was added.
			if meta.IsNoMatchError(err) {
				log.Error(err, "An error occurred when initializing extension controllers, will retry")
				return retry.MinorError(err)
			}
			return retry.SevereError(err)
		}
		return retry.Ok()
	}); err != nil {
		cancel()
		return err
	}

	go extensionsController.Run(controllerCtx, *f.Config.Controllers.ControllerInstallationRequired.ConcurrentSyncs, *f.Config.Controllers.ShootStateSync.ConcurrentSyncs)

	log.Info("gardenlet initialized")

	// block until shutting down
	<-ctx.Done()
	cancel()
	return nil
}
