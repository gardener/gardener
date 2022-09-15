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
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
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
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	schedulingv1 "k8s.io/api/scheduling/v1"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultImageVector is a constant for the path to the default image vector file.
const DefaultImageVector = "images.yaml"

// GardenletControllerFactory contains information relevant to controllers for the Garden API group.
type GardenletControllerFactory struct {
	log                                  logr.Logger
	clientMap                            clientmap.ClientMap
	cfg                                  *config.GardenletConfiguration
	identity                             *gardencorev1beta1.Gardener
	gardenClusterIdentity                string
	recorder                             record.EventRecorder
	healthManager                        healthz.Manager
	clientCertificateExpirationTimestamp *metav1.Time
}

// NewGardenletControllerFactory creates a new factory for controllers for the Garden API group.
func NewGardenletControllerFactory(
	log logr.Logger,
	clientMap clientmap.ClientMap,
	cfg *config.GardenletConfiguration,
	identity *gardencorev1beta1.Gardener,
	gardenClusterIdentity string,
	recorder record.EventRecorder,
	healthManager healthz.Manager,
	clientCertificateExpirationTimestamp *metav1.Time,
) *GardenletControllerFactory {
	return &GardenletControllerFactory{
		log:                                  log,
		clientMap:                            clientMap,
		cfg:                                  cfg,
		identity:                             identity,
		gardenClusterIdentity:                gardenClusterIdentity,
		recorder:                             recorder,
		healthManager:                        healthManager,
		clientCertificateExpirationTimestamp: clientCertificateExpirationTimestamp,
	}
}

// Run starts all the controllers for the Garden API group. It also performs bootstrapping tasks.
func (f *GardenletControllerFactory) Run(ctx context.Context) error {
	log := f.log.WithName("controller")

	gardenClient, err := f.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %+v", err)
	}

	if err := addAllGardenFieldIndexes(ctx, gardenClient.Cache()); err != nil {
		return fmt.Errorf("failed adding indexes: %w", err)
	}

	if err := f.clientMap.Start(ctx.Done()); err != nil {
		return fmt.Errorf("failed to start ClientMap: %+v", err)
	}

	// Register Seed object
	if err := f.registerSeed(ctx, gardenClient.Client()); err != nil {
		return fmt.Errorf("failed to register the seed: %+v", err)
	}

	imageVector, err := imagevector.ReadGlobalImageVectorWithEnvOverride(filepath.Join(charts.Path, DefaultImageVector))
	runtime.Must(err)

	var componentImageVectors imagevector.ComponentImageVectors
	if path := os.Getenv(imagevector.ComponentOverrideEnv); path != "" {
		componentImageVectors, err = imagevector.ReadComponentOverwriteFile(path)
		runtime.Must(err)
	}

	gardenNamespace := &corev1.Namespace{}
	runtime.Must(gardenClient.Client().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace), gardenNamespace))

	seedClient, err := f.clientMap.GetClient(ctx, keys.ForSeedWithName(f.cfg.SeedConfig.Name))
	if err != nil {
		return fmt.Errorf("failed to get seed client: %w", err)
	}

	// TODO(acumino): Remove in a future release.
	if err := client.IgnoreNotFound(seedClient.Client().Delete(ctx, &schedulingv1.PriorityClass{ObjectMeta: metav1.ObjectMeta{Name: "gardener-system-critical-migration"}})); err != nil {
		return fmt.Errorf("unable to delete Gardenlet's old PriorityClass: %w", err)
	}

	backupBucketController, err := backupbucketcontroller.NewBackupBucketController(ctx, log, f.clientMap, f.cfg, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing BackupBucket controller: %w", err)
	}

	backupEntryController, err := backupentrycontroller.NewBackupEntryController(ctx, log, f.clientMap, f.cfg, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing BackupEntry controller: %w", err)
	}

	bastionController, err := bastioncontroller.NewBastionController(ctx, log, f.clientMap, f.cfg)
	if err != nil {
		return fmt.Errorf("failed initializing Bastion controller: %w", err)
	}

	controllerInstallationController, err := controllerinstallationcontroller.NewController(ctx, log, f.clientMap, f.cfg, f.identity, gardenNamespace, f.gardenClusterIdentity)
	if err != nil {
		return fmt.Errorf("failed initializing ControllerInstallation controller: %w", err)
	}

	extensionsController := extensionscontroller.NewController(log, gardenClient.Client(), seedClient.Client(), f.cfg.SeedConfig.Name)

	managedSeedController, err := managedseedcontroller.NewManagedSeedController(ctx, log, f.clientMap, f.cfg, imageVector, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing ManagedSeed controller: %w", err)
	}

	networkPolicyController, err := networkpolicycontroller.NewController(ctx, log, seedClient, f.cfg.SeedConfig.Name)
	if err != nil {
		return fmt.Errorf("failed initializing NetworkPolicy controller: %w", err)
	}

	secretController, err := shootsecretcontroller.NewController(ctx, log, gardenClient.Client(), seedClient)
	if err != nil {
		return fmt.Errorf("failed initializing Secret controller: %w", err)
	}

	seedController, err := seedcontroller.NewSeedController(ctx, log, f.clientMap, f.healthManager, imageVector, componentImageVectors, f.identity, f.clientCertificateExpirationTimestamp, f.cfg, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing Seed controller: %w", err)
	}

	shootController, err := shootcontroller.NewShootController(ctx, log, f.clientMap, f.cfg, f.identity, f.gardenClusterIdentity, imageVector, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing Shoot controller: %w", err)
	}

	controllerCtx, cancel := context.WithCancel(ctx)

	go backupBucketController.Run(controllerCtx, *f.cfg.Controllers.BackupBucket.ConcurrentSyncs)
	go backupEntryController.Run(controllerCtx, *f.cfg.Controllers.BackupEntry.ConcurrentSyncs, *f.cfg.Controllers.BackupEntryMigration.ConcurrentSyncs)
	go bastionController.Run(controllerCtx, *f.cfg.Controllers.Bastion.ConcurrentSyncs)
	go controllerInstallationController.Run(controllerCtx, *f.cfg.Controllers.ControllerInstallation.ConcurrentSyncs, *f.cfg.Controllers.ControllerInstallationCare.ConcurrentSyncs)
	go managedSeedController.Run(controllerCtx, *f.cfg.Controllers.ManagedSeed.ConcurrentSyncs)
	go networkPolicyController.Run(controllerCtx, *f.cfg.Controllers.SeedAPIServerNetworkPolicy.ConcurrentSyncs)
	go secretController.Run(controllerCtx, *f.cfg.Controllers.ShootSecret.ConcurrentSyncs)
	go seedController.Run(controllerCtx, *f.cfg.Controllers.Seed.ConcurrentSyncs)
	go shootController.Run(controllerCtx, *f.cfg.Controllers.Shoot.ConcurrentSyncs, *f.cfg.Controllers.ShootCare.ConcurrentSyncs, *f.cfg.Controllers.ShootMigration.ConcurrentSyncs)

	// TODO(timebertt): this can be removed once we have refactored gardenlet to native controller-runtime controllers,
	// with https://github.com/kubernetes-sigs/controller-runtime/pull/1678 source.Kind already retries getting
	// an informer on NoKindMatch (just make sure, /readyz fails until we have an informer)
	if err := retry.Until(ctx, 10*time.Second, func(ctx context.Context) (bool, error) {
		if err := extensionsController.Initialize(ctx, seedClient); err != nil {
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

	go extensionsController.Run(controllerCtx, *f.cfg.Controllers.ControllerInstallationRequired.ConcurrentSyncs, *f.cfg.Controllers.ShootStateSync.ConcurrentSyncs)

	log.Info("gardenlet initialized")

	// Shutdown handling
	<-ctx.Done()
	cancel()

	log.Info("I have received a stop signal and will no longer watch resources")
	log.Info("Bye Bye!")

	return nil
}
