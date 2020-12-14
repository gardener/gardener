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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	gardenmetrics "github.com/gardener/gardener/pkg/controllerutils/metrics"
	"github.com/gardener/gardener/pkg/gardenlet"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	backupbucketcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/backupbucket"
	backupentrycontroller "github.com/gardener/gardener/pkg/gardenlet/controller/backupentry"
	controllerinstallationcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation"
	federatedseedcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/federatedseed"
	seedcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/seed"
	shootcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/shoot"
	"github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/version"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

// DefaultImageVector is a constant for the path to the default image vector file.
const DefaultImageVector = "images.yaml"

// GardenletControllerFactory contains information relevant to controllers for the Garden API group.
type GardenletControllerFactory struct {
	cfg                    *config.GardenletConfiguration
	gardenClusterIdentity  string
	identity               *gardencorev1beta1.Gardener
	gardenNamespace        string
	clientMap              clientmap.ClientMap
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory
	k8sInformers           kubeinformers.SharedInformerFactory
	recorder               record.EventRecorder
	healthManager          healthz.Manager
}

// NewGardenletControllerFactory creates a new factory for controllers for the Garden API group.
func NewGardenletControllerFactory(
	clientMap clientmap.ClientMap,
	gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	cfg *config.GardenletConfiguration, identity *gardencorev1beta1.Gardener,
	gardenClusterIdentity, gardenNamespace string,
	recorder record.EventRecorder,
	healthManager healthz.Manager,
) *GardenletControllerFactory {
	return &GardenletControllerFactory{
		cfg:                    cfg,
		identity:               identity,
		gardenClusterIdentity:  gardenClusterIdentity,
		gardenNamespace:        gardenNamespace,
		clientMap:              clientMap,
		k8sGardenCoreInformers: gardenCoreInformerFactory,
		k8sInformers:           kubeInformerFactory,
		recorder:               recorder,
		healthManager:          healthManager,
	}
}

// Run starts all the controllers for the Garden API group. It also performs bootstrapping tasks.
func (f *GardenletControllerFactory) Run(ctx context.Context) error {
	var (
		// Garden core informers
		backupBucketInformer           = f.k8sGardenCoreInformers.Core().V1beta1().BackupBuckets().Informer()
		backupEntryInformer            = f.k8sGardenCoreInformers.Core().V1beta1().BackupEntries().Informer()
		cloudProfileInformer           = f.k8sGardenCoreInformers.Core().V1beta1().CloudProfiles().Informer()
		controllerRegistrationInformer = f.k8sGardenCoreInformers.Core().V1beta1().ControllerRegistrations().Informer()
		controllerInstallationInformer = f.k8sGardenCoreInformers.Core().V1beta1().ControllerInstallations().Informer()
		projectInformer                = f.k8sGardenCoreInformers.Core().V1beta1().Projects().Informer()
		secretBindingInformer          = f.k8sGardenCoreInformers.Core().V1beta1().SecretBindings().Informer()
		seedInformer                   = f.k8sGardenCoreInformers.Core().V1beta1().Seeds().Informer()
		shootInformer                  = f.k8sGardenCoreInformers.Core().V1beta1().Shoots().Informer()
		// Kubernetes core informers
		namespaceInformer = f.k8sInformers.Core().V1().Namespaces().Informer()
		secretInformer    = f.k8sInformers.Core().V1().Secrets().Informer()
		configMapInformer = f.k8sInformers.Core().V1().ConfigMaps().Informer()
	)

	if err := f.clientMap.Start(ctx.Done()); err != nil {
		return fmt.Errorf("failed to start ClientMap: %+v", err)
	}

	k8sGardenClient, err := f.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %+v", err)
	}

	f.k8sGardenCoreInformers.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), backupBucketInformer.HasSynced, backupEntryInformer.HasSynced, cloudProfileInformer.HasSynced, controllerRegistrationInformer.HasSynced, controllerInstallationInformer.HasSynced, projectInformer.HasSynced, secretBindingInformer.HasSynced, seedInformer.HasSynced, shootInformer.HasSynced) {
		return fmt.Errorf("timed out waiting for Garden core caches to sync")
	}

	f.k8sInformers.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), namespaceInformer.HasSynced, secretInformer.HasSynced, configMapInformer.HasSynced) {
		return fmt.Errorf("timed out waiting for Kube caches to sync")
	}

	secrets, err := garden.ReadGardenSecrets(f.k8sInformers, f.k8sGardenCoreInformers)
	runtime.Must(err)

	if secret, ok := secrets[common.GardenRoleInternalDomain]; ok {
		shootList, err := f.k8sGardenCoreInformers.Core().V1beta1().Shoots().Lister().List(labels.Everything())
		runtime.Must(err)
		runtime.Must(garden.VerifyInternalDomainSecret(k8sGardenClient, len(shootList), secret))
	}

	imageVector, err := imagevector.ReadGlobalImageVectorWithEnvOverride(filepath.Join(common.ChartPath, DefaultImageVector))
	runtime.Must(err)

	var componentImageVectors imagevector.ComponentImageVectors
	if path := os.Getenv(imagevector.ComponentOverrideEnv); path != "" {
		componentImageVectors, err = imagevector.ReadComponentOverwriteFile(path)
		runtime.Must(err)
	}

	gardenNamespace := &corev1.Namespace{}
	// Use direct client here since we don't want to cache all namespaces of the Garden cluster.
	runtime.Must(k8sGardenClient.DirectClient().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace), gardenNamespace))

	// Initialize the workqueue metrics collection.
	gardenmetrics.RegisterWorkqueMetrics()

	var (
		controllerInstallationController = controllerinstallationcontroller.NewController(f.clientMap, f.k8sGardenCoreInformers, f.cfg, f.recorder, gardenNamespace, f.gardenClusterIdentity)
		seedController                   = seedcontroller.NewSeedController(f.clientMap, f.k8sGardenCoreInformers, f.k8sInformers, f.healthManager, secrets, imageVector, componentImageVectors, f.identity, f.cfg, f.recorder)
		shootController                  = shootcontroller.NewShootController(f.clientMap, f.k8sGardenCoreInformers, f.cfg, f.identity, f.gardenClusterIdentity, secrets, imageVector, f.recorder)
	)

	backupBucketController, err := backupbucketcontroller.NewBackupBucketController(ctx, f.clientMap, f.cfg, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing BackupBucket controller: %w", err)
	}

	backupEntryController, err := backupentrycontroller.NewBackupEntryController(ctx, f.clientMap, f.cfg, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing BackupEntry controller: %w", err)
	}

	federatedSeedController, err := federatedseedcontroller.NewFederatedSeedController(ctx, f.clientMap, f.cfg, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing federated seed controller: %w", err)
	}

	// Initialize the Controller metrics collection.
	gardenmetrics.RegisterControllerMetrics(
		gardenlet.ControllerWorkerSum,
		gardenlet.ScrapeFailures,
		backupBucketController,
		backupEntryController,
		controllerInstallationController,
		seedController,
		shootController,
	)

	go federatedSeedController.Run(ctx, *f.cfg.Controllers.Seed.ConcurrentSyncs)
	go backupBucketController.Run(ctx, *f.cfg.Controllers.BackupBucket.ConcurrentSyncs)
	go backupEntryController.Run(ctx, *f.cfg.Controllers.BackupEntry.ConcurrentSyncs)
	go controllerInstallationController.Run(ctx, *f.cfg.Controllers.ControllerInstallation.ConcurrentSyncs, *f.cfg.Controllers.ControllerInstallationCare.ConcurrentSyncs)
	go seedController.Run(ctx, *f.cfg.Controllers.Seed.ConcurrentSyncs)
	go shootController.Run(ctx, *f.cfg.Controllers.Shoot.ConcurrentSyncs, *f.cfg.Controllers.ShootCare.ConcurrentSyncs)

	logger.Logger.Infof("Gardenlet (version %s) initialized.", version.Get().GitVersion)

	// Shutdown handling
	<-ctx.Done()

	logger.Logger.Infof("I have received a stop signal and will no longer watch resources.")
	logger.Logger.Infof("Bye Bye!")

	return nil
}
