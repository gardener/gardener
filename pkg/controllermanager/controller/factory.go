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
	"path/filepath"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	backupinfrastructurecontroller "github.com/gardener/gardener/pkg/controllermanager/controller/backupinfrastructure"
	cloudprofilecontroller "github.com/gardener/gardener/pkg/controllermanager/controller/cloudprofile"
	controllerinstallationcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/controllerinstallation"
	controllerregistrationcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration"
	projectcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/project"
	quotacontroller "github.com/gardener/gardener/pkg/controllermanager/controller/quota"
	secretbindingcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/secretbinding"
	seedcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/seed"
	shootcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/shoot"
	gardenmetrics "github.com/gardener/gardener/pkg/controllermanager/metrics"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/version"

	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

// GardenControllerFactory contains information relevant to controllers for the Garden API group.
type GardenControllerFactory struct {
	cfg                    *config.ControllerManagerConfiguration
	identity               *gardenv1beta1.Gardener
	gardenNamespace        string
	k8sGardenClient        kubernetes.Interface
	k8sGardenInformers     gardeninformers.SharedInformerFactory
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory
	k8sInformers           kubeinformers.SharedInformerFactory
	recorder               record.EventRecorder
}

// NewGardenControllerFactory creates a new factory for controllers for the Garden API group.
func NewGardenControllerFactory(k8sGardenClient kubernetes.Interface, gardenInformerFactory gardeninformers.SharedInformerFactory, gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory, kubeInformerFactory kubeinformers.SharedInformerFactory, cfg *config.ControllerManagerConfiguration, identity *gardenv1beta1.Gardener, gardenNamespace string, recorder record.EventRecorder) *GardenControllerFactory {
	return &GardenControllerFactory{
		cfg:                    cfg,
		identity:               identity,
		gardenNamespace:        gardenNamespace,
		k8sGardenClient:        k8sGardenClient,
		k8sGardenInformers:     gardenInformerFactory,
		k8sGardenCoreInformers: gardenCoreInformerFactory,
		k8sInformers:           kubeInformerFactory,
		recorder:               recorder,
	}
}

// Run starts all the controllers for the Garden API group. It also performs bootstrapping tasks.
func (f *GardenControllerFactory) Run(ctx context.Context) {
	var (
		cloudProfileInformer           = f.k8sGardenInformers.Garden().V1beta1().CloudProfiles().Informer()
		secretBindingInformer          = f.k8sGardenInformers.Garden().V1beta1().SecretBindings().Informer()
		quotaInformer                  = f.k8sGardenInformers.Garden().V1beta1().Quotas().Informer()
		projectInformer                = f.k8sGardenInformers.Garden().V1beta1().Projects().Informer()
		seedInformer                   = f.k8sGardenInformers.Garden().V1beta1().Seeds().Informer()
		shootInformer                  = f.k8sGardenInformers.Garden().V1beta1().Shoots().Informer()
		backupInfrastructureInformer   = f.k8sGardenInformers.Garden().V1beta1().BackupInfrastructures().Informer()
		controllerRegistrationInformer = f.k8sGardenCoreInformers.Core().V1alpha1().ControllerRegistrations().Informer()
		controllerInstallationInformer = f.k8sGardenCoreInformers.Core().V1alpha1().ControllerInstallations().Informer()

		namespaceInformer = f.k8sInformers.Core().V1().Namespaces().Informer()
		secretInformer    = f.k8sInformers.Core().V1().Secrets().Informer()
		configMapInformer = f.k8sInformers.Core().V1().ConfigMaps().Informer()
	)

	f.k8sGardenInformers.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), cloudProfileInformer.HasSynced, secretBindingInformer.HasSynced, quotaInformer.HasSynced, projectInformer.HasSynced, seedInformer.HasSynced, shootInformer.HasSynced, backupInfrastructureInformer.HasSynced) {
		panic("Timed out waiting for Garden caches to sync")
	}

	f.k8sGardenCoreInformers.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), controllerRegistrationInformer.HasSynced, controllerInstallationInformer.HasSynced) {
		panic("Timed out waiting for Garden core caches to sync")
	}

	f.k8sInformers.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), namespaceInformer.HasSynced, secretInformer.HasSynced, configMapInformer.HasSynced) {
		panic("Timed out waiting for Kube caches to sync")
	}

	secrets, err := garden.ReadGardenSecrets(f.k8sInformers)
	if err != nil {
		panic(err)
	}
	shootList, err := f.k8sGardenInformers.Garden().V1beta1().Shoots().Lister().List(labels.Everything())
	if err != nil {
		panic(err)
	}
	if err := garden.VerifyInternalDomainSecret(f.k8sGardenClient, len(shootList), secrets[common.GardenRoleInternalDomain]); err != nil {
		panic(err)
	}

	imageVector, err := imagevector.ReadImageVector(filepath.Join(common.ChartPath, "images.yaml"))
	if err != nil {
		panic(err)
	}

	if err := garden.BootstrapCluster(f.k8sGardenClient, common.GardenNamespace, secrets); err != nil {
		logger.Logger.Errorf("Failed to bootstrap the Garden cluster: %s", err.Error())
		return
	}
	logger.Logger.Info("Successfully bootstrapped the Garden cluster.")

	// Initialize the workqueue metrics collection.
	gardenmetrics.RegisterWorkqueMetrics()

	var (
		shootController                  = shootcontroller.NewShootController(f.k8sGardenClient, f.k8sGardenInformers, f.k8sGardenCoreInformers, f.k8sInformers, f.cfg, f.identity, f.gardenNamespace, secrets, imageVector, f.recorder)
		seedController                   = seedcontroller.NewSeedController(f.k8sGardenClient, f.k8sGardenInformers, f.k8sInformers, secrets, imageVector, f.cfg, f.recorder)
		quotaController                  = quotacontroller.NewQuotaController(f.k8sGardenClient, f.k8sGardenInformers, f.recorder)
		projectController                = projectcontroller.NewProjectController(f.k8sGardenClient, f.k8sGardenInformers, f.k8sInformers, f.recorder)
		cloudProfileController           = cloudprofilecontroller.NewCloudProfileController(f.k8sGardenClient, f.k8sGardenInformers)
		secretBindingController          = secretbindingcontroller.NewSecretBindingController(f.k8sGardenClient, f.k8sGardenInformers, f.k8sInformers, f.recorder)
		backupInfrastructureController   = backupinfrastructurecontroller.NewBackupInfrastructureController(f.k8sGardenClient, f.k8sGardenInformers, f.cfg, f.identity, f.gardenNamespace, secrets, imageVector, f.recorder)
		controllerRegistrationController = controllerregistrationcontroller.NewController(f.k8sGardenClient, f.k8sGardenInformers, f.k8sGardenCoreInformers, f.cfg, f.recorder)
		controllerInstallationController = controllerinstallationcontroller.NewController(f.k8sGardenClient, f.k8sGardenInformers, f.k8sGardenCoreInformers, f.cfg, f.recorder)
	)

	// Initialize the Controller metrics collection.
	gardenmetrics.RegisterControllerMetrics(shootController, seedController, quotaController, cloudProfileController, secretBindingController, backupInfrastructureController)

	go shootController.Run(ctx, f.cfg.Controllers.Shoot.ConcurrentSyncs, f.cfg.Controllers.ShootCare.ConcurrentSyncs, f.cfg.Controllers.ShootMaintenance.ConcurrentSyncs, f.cfg.Controllers.ShootQuota.ConcurrentSyncs, f.cfg.Controllers.ShootHibernation.ConcurrentSyncs)
	go seedController.Run(ctx, f.cfg.Controllers.Seed.ConcurrentSyncs)
	go quotaController.Run(ctx, f.cfg.Controllers.Quota.ConcurrentSyncs)
	go projectController.Run(ctx, f.cfg.Controllers.Project.ConcurrentSyncs)
	go cloudProfileController.Run(ctx, f.cfg.Controllers.CloudProfile.ConcurrentSyncs)
	go secretBindingController.Run(ctx, f.cfg.Controllers.SecretBinding.ConcurrentSyncs)
	go backupInfrastructureController.Run(ctx, f.cfg.Controllers.BackupInfrastructure.ConcurrentSyncs)
	go controllerRegistrationController.Run(ctx, f.cfg.Controllers.ControllerRegistration.ConcurrentSyncs)
	go controllerInstallationController.Run(ctx, f.cfg.Controllers.ControllerInstallation.ConcurrentSyncs)

	logger.Logger.Infof("Gardener controller manager (version %s) initialized.", version.Get().GitVersion)

	// Shutdown handling
	<-ctx.Done()

	logger.Logger.Infof("I have received a stop signal and will no longer watch events of the Garden API group.")
	logger.Logger.Infof("Bye Bye!")
}
