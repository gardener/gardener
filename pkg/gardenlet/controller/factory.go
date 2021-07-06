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
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
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
	federatedseedcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/federatedseed"
	managedseedcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/managedseed"
	seedcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/seed"
	shootcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/shoot"
	"github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/component-base/version"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// DefaultImageVector is a constant for the path to the default image vector file.
const DefaultImageVector = "images.yaml"

// GardenletControllerFactory contains information relevant to controllers for the Garden API group.
type GardenletControllerFactory struct {
	cfg                    *config.GardenletConfiguration
	gardenClusterIdentity  string
	identity               *gardencorev1beta1.Gardener
	clientMap              clientmap.ClientMap
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory
	recorder               record.EventRecorder
	healthManager          healthz.Manager
}

// NewGardenletControllerFactory creates a new factory for controllers for the Garden API group.
func NewGardenletControllerFactory(
	clientMap clientmap.ClientMap,
	gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory,
	cfg *config.GardenletConfiguration,
	identity *gardencorev1beta1.Gardener,
	gardenClusterIdentity string,
	recorder record.EventRecorder,
	healthManager healthz.Manager,
) *GardenletControllerFactory {
	return &GardenletControllerFactory{
		cfg:                    cfg,
		identity:               identity,
		gardenClusterIdentity:  gardenClusterIdentity,
		clientMap:              clientMap,
		k8sGardenCoreInformers: gardenCoreInformerFactory,
		recorder:               recorder,
		healthManager:          healthManager,
	}
}

// Run starts all the controllers for the Garden API group. It also performs bootstrapping tasks.
func (f *GardenletControllerFactory) Run(ctx context.Context) error {
	var (
		controllerRegistrationInformer = f.k8sGardenCoreInformers.Core().V1beta1().ControllerRegistrations().Informer()
		controllerInstallationInformer = f.k8sGardenCoreInformers.Core().V1beta1().ControllerInstallations().Informer()
		seedInformer                   = f.k8sGardenCoreInformers.Core().V1beta1().Seeds().Informer()
		shootInformer                  = f.k8sGardenCoreInformers.Core().V1beta1().Shoots().Informer()
	)

	if err := f.clientMap.Start(ctx.Done()); err != nil {
		return fmt.Errorf("failed to start ClientMap: %+v", err)
	}

	k8sGardenClient, err := f.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %+v", err)
	}

	f.k8sGardenCoreInformers.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), controllerRegistrationInformer.HasSynced, controllerInstallationInformer.HasSynced, seedInformer.HasSynced, shootInformer.HasSynced) {
		return fmt.Errorf("timed out waiting for Garden core caches to sync")
	}

	// Register Seed object
	if err := f.registerSeed(ctx, k8sGardenClient); err != nil {
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
	runtime.Must(k8sGardenClient.Client().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace), gardenNamespace))

	// Initialize the workqueue metrics collection.
	gardenmetrics.RegisterWorkqueMetrics()

	var (
		controllerInstallationController = controllerinstallationcontroller.NewController(f.clientMap, f.k8sGardenCoreInformers, f.cfg, f.recorder, gardenNamespace, f.gardenClusterIdentity)
		seedController                   = seedcontroller.NewSeedController(f.clientMap, f.k8sGardenCoreInformers, f.healthManager, imageVector, componentImageVectors, f.identity, f.cfg, f.recorder)
		shootController                  = shootcontroller.NewShootController(f.clientMap, f.k8sGardenCoreInformers, f.cfg, f.identity, f.gardenClusterIdentity, imageVector, f.recorder)
	)

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

	federatedSeedController, err := federatedseedcontroller.NewFederatedSeedController(ctx, f.clientMap, f.cfg, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing federated seed controller: %w", err)
	}

	managedSeedController, err := managedseedcontroller.NewManagedSeedController(ctx, f.clientMap, f.cfg, imageVector, f.recorder, logger.Logger)
	if err != nil {
		return fmt.Errorf("failed initializing managed seed controller: %w", err)
	}

	// Initialize the Controller metrics collection.
	gardenmetrics.RegisterControllerMetrics(
		gardenlet.ControllerWorkerSum,
		gardenlet.ScrapeFailures,
		backupBucketController,
		backupEntryController,
		bastionController,
		controllerInstallationController,
		seedController,
		shootController,
		managedSeedController,
	)

	go federatedSeedController.Run(ctx, *f.cfg.Controllers.Seed.ConcurrentSyncs)
	go backupBucketController.Run(ctx, *f.cfg.Controllers.BackupBucket.ConcurrentSyncs)
	go backupEntryController.Run(ctx, *f.cfg.Controllers.BackupEntry.ConcurrentSyncs)
	go bastionController.Run(ctx, *f.cfg.Controllers.Bastion.ConcurrentSyncs)
	go controllerInstallationController.Run(ctx, *f.cfg.Controllers.ControllerInstallation.ConcurrentSyncs, *f.cfg.Controllers.ControllerInstallationCare.ConcurrentSyncs)
	go seedController.Run(ctx, *f.cfg.Controllers.Seed.ConcurrentSyncs)
	go shootController.Run(ctx, *f.cfg.Controllers.Shoot.ConcurrentSyncs, *f.cfg.Controllers.ShootCare.ConcurrentSyncs)
	go managedSeedController.Run(ctx, *f.cfg.Controllers.ManagedSeed.ConcurrentSyncs)

	logger.Logger.Infof("Gardenlet (version %s) initialized.", version.Get().GitVersion)

	// Shutdown handling
	<-ctx.Done()

	logger.Logger.Infof("I have received a stop signal and will no longer watch resources.")
	logger.Logger.Infof("Bye Bye!")

	return nil
}

// registerSeed reconciles the seed resource if gardenlet is configured to take care about it.
func (f *GardenletControllerFactory) registerSeed(ctx context.Context, gardenClient kubernetes.Interface) error {
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

	operationResult, err := controllerutils.GetAndCreateOrMergePatch(ctx, gardenClient.Client(), seed, func() error {
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
	return gardenClient.Client().Get(ctx, kutil.Key(gutil.ComputeGardenNamespace(f.cfg.SeedConfig.Name)), &corev1.Namespace{})
}
