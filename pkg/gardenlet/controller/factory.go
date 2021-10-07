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
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenmetrics "github.com/gardener/gardener/pkg/controllerutils/metrics"
	"github.com/gardener/gardener/pkg/gardenlet"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	backupbucketcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/backupbucket"
	backupentrycontroller "github.com/gardener/gardener/pkg/gardenlet/controller/backupentry"
	bastioncontroller "github.com/gardener/gardener/pkg/gardenlet/controller/bastion"
	controllerinstallationcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation"
	extensionscontroller "github.com/gardener/gardener/pkg/gardenlet/controller/extensions"
	managedseedcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/managedseed"
	networkpolicycontroller "github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy"
	seedcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/seed"
	shootcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/shoot"
	"github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/component-base/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// DefaultImageVector is a constant for the path to the default image vector file.
const DefaultImageVector = "images.yaml"

// GardenletControllerFactory contains information relevant to controllers for the Garden API group.
type GardenletControllerFactory struct {
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
	clientMap clientmap.ClientMap,
	cfg *config.GardenletConfiguration,
	identity *gardencorev1beta1.Gardener,
	gardenClusterIdentity string,
	recorder record.EventRecorder,
	healthManager healthz.Manager,
	clientCertificateExpirationTimestamp *metav1.Time,
) *GardenletControllerFactory {
	return &GardenletControllerFactory{
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
	gardenClientSet, err := f.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %+v", err)
	}

	if err := addAllFieldIndexes(ctx, gardenClientSet.Cache()); err != nil {
		return err
	}

	if err := f.clientMap.Start(ctx.Done()); err != nil {
		return fmt.Errorf("failed to start ClientMap: %+v", err)
	}

	// Register Seed object
	if err := f.registerSeed(ctx, gardenClientSet.Client()); err != nil {
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
	runtime.Must(gardenClientSet.Client().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace), gardenNamespace))

	// Initialize the workqueue metrics collection.
	gardenmetrics.RegisterWorkqueMetrics()

	seedName := f.cfg.SeedConfig.Name
	seedClient, err := f.clientMap.GetClient(ctx, keys.ForSeedWithName(seedName))
	if err != nil {
		return fmt.Errorf("failed to get seed client: %w", err)
	}

	backupBucketController, err := backupbucketcontroller.NewBackupBucketController(ctx, f.clientMap, f.cfg, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing BackupBucket controller: %w", err)
	}

	backupEntryController, err := backupentrycontroller.NewBackupEntryController(ctx, f.clientMap, f.cfg, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing BackupEntry controller: %w", err)
	}

	bastionController, err := bastioncontroller.NewBastionController(ctx, f.clientMap, f.cfg)
	if err != nil {
		return fmt.Errorf("failed initializing Bastion controller: %w", err)
	}

	controllerInstallationController, err := controllerinstallationcontroller.NewController(ctx, f.clientMap, f.cfg, f.recorder, gardenNamespace, f.gardenClusterIdentity)
	if err != nil {
		return fmt.Errorf("failed initializing ControllerInstallation controller: %w", err)
	}

	extensionsController := extensionscontroller.NewController(gardenClientSet, seedClient, f.cfg.SeedConfig.Name, logger.Logger, f.recorder)

	managedSeedController, err := managedseedcontroller.NewManagedSeedController(ctx, f.clientMap, f.cfg, imageVector, f.recorder, logger.Logger)
	if err != nil {
		return fmt.Errorf("failed initializing ManagedSeed controller: %w", err)
	}

	networkPolicyController, err := networkpolicycontroller.NewController(ctx, seedClient, logger.Logger, f.recorder, f.cfg.SeedConfig.Name)
	if err != nil {
		return fmt.Errorf("failed initializing NetworkPolicy controller: %w", err)
	}

	seedController, err := seedcontroller.NewSeedController(ctx, f.clientMap, f.healthManager, imageVector, componentImageVectors, f.identity, f.clientCertificateExpirationTimestamp, f.cfg, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing Seed controller: %w", err)
	}

	shootController, err := shootcontroller.NewShootController(ctx, f.clientMap, logger.Logger, f.cfg, f.identity, f.gardenClusterIdentity, imageVector, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing Shoot controller: %w", err)
	}

	// Initialize the Controller metrics collection.
	gardenmetrics.RegisterControllerMetrics(
		gardenlet.ControllerWorkerSum,
		gardenlet.ScrapeFailures,
		backupBucketController,
		backupEntryController,
		bastionController,
		controllerInstallationController,
		extensionsController,
		managedSeedController,
		networkPolicyController,
		seedController,
		shootController,
	)

	controllerCtx, cancel := context.WithCancel(ctx)

	go backupBucketController.Run(controllerCtx, *f.cfg.Controllers.BackupBucket.ConcurrentSyncs)
	go backupEntryController.Run(controllerCtx, *f.cfg.Controllers.BackupEntry.ConcurrentSyncs)
	go bastionController.Run(controllerCtx, *f.cfg.Controllers.Bastion.ConcurrentSyncs)
	go controllerInstallationController.Run(controllerCtx, *f.cfg.Controllers.ControllerInstallation.ConcurrentSyncs, *f.cfg.Controllers.ControllerInstallationCare.ConcurrentSyncs)
	go managedSeedController.Run(controllerCtx, *f.cfg.Controllers.ManagedSeed.ConcurrentSyncs)
	go networkPolicyController.Run(controllerCtx, *f.cfg.Controllers.SeedAPIServerNetworkPolicy.ConcurrentSyncs)
	go seedController.Run(controllerCtx, *f.cfg.Controllers.Seed.ConcurrentSyncs)
	go shootController.Run(controllerCtx, *f.cfg.Controllers.Shoot.ConcurrentSyncs, *f.cfg.Controllers.ShootCare.ConcurrentSyncs)

	if err := retry.Until(ctx, 10*time.Second, func(ctx context.Context) (bool, error) {
		if err := extensionsController.Initialize(ctx, seedClient); err != nil {
			// A NoMatchError most probably indicates that the necessary CRDs haven't been deployed to the affected seed cluster yet.
			// This can either be the case if the seed cluster is new or if a new extension CRD was added.
			if meta.IsNoMatchError(err) {
				logger.Logger.Errorf("An error occurred when initializing extension controllers: %v. Will retry.", err)
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

	logger.Logger.Infof("Gardenlet (version %s) initialized.", version.Get().GitVersion)

	// Shutdown handling
	<-ctx.Done()
	cancel()

	logger.Logger.Infof("I have received a stop signal and will no longer watch resources.")
	logger.Logger.Infof("Bye Bye!")

	return nil
}

// registerSeed reconciles the seed resource if gardenlet is configured to take care about it.
func (f *GardenletControllerFactory) registerSeed(ctx context.Context, gardenClient client.Client) error {
	seed := &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name: f.cfg.SeedConfig.Name,
		},
	}

	// Convert gardenlet config to an external version
	cfg, err := confighelper.ConvertGardenletConfigurationExternal(f.cfg)
	if err != nil {
		return fmt.Errorf("could not convert gardenlet configuration: %+v", err)
	}

	operationResult, err := controllerutils.GetAndCreateOrMergePatch(ctx, gardenClient, seed, func() error {
		seed.Labels = utils.MergeStringMaps(map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleSeed,
		}, f.cfg.SeedConfig.Labels)

		seed.Spec = cfg.SeedConfig.Spec
		return nil
	})
	if err != nil {
		return fmt.Errorf("could not register seed %q: %+v", seed.Name, err)
	}

	// If the Seed was freshly created then the `seed-<name>` Namespace does not yet exist. It will be created by the
	// gardener-controller-manager (GCM). If the SeedAuthorizer is enabled then the gardenlet might fail/exit with an
	// error if it GETs the Namespace too fast (before GCM created it), hence, let's wait to give GCM some time to
	// create it.
	if operationResult == controllerutil.OperationResultCreated {
		time.Sleep(5 * time.Second)
	}

	// Verify that seed namespace exists.
	return gardenClient.Get(ctx, kutil.Key(gutil.ComputeGardenNamespace(f.cfg.SeedConfig.Name)), &corev1.Namespace{})
}

// addAllFieldIndexes adds all field indexes used by gardenlet to the given FieldIndexer (i.e. cache).
// field indexes have to be added before the cache is started (i.e. before the clientmap is started)
func addAllFieldIndexes(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &gardencorev1beta1.ControllerInstallation{}, gardencore.SeedRefName, func(obj client.Object) []string {
		controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
		if !ok {
			return []string{""}
		}
		return []string{controllerInstallation.Spec.SeedRef.Name}
	}); err != nil {
		return fmt.Errorf("failed to add indexer to ControllerInstallation Informer: %w", err)
	}

	if err := indexer.IndexField(ctx, &seedmanagementv1alpha1.ManagedSeed{}, seedmanagement.ManagedSeedShootName, func(obj client.Object) []string {
		managedSeed, ok := obj.(*seedmanagementv1alpha1.ManagedSeed)
		if !ok {
			return []string{""}
		}
		if shoot := managedSeed.Spec.Shoot; shoot != nil {
			return []string{managedSeed.Spec.Shoot.Name}
		}
		return []string{""}
	}); err != nil {
		return fmt.Errorf("failed to add indexer to ManagedSeed Informer: %w", err)
	}

	return nil
}
