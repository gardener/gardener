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
	"text/tabwriter"

	"github.com/gardener/gardener/pkg/apis/componentconfig"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	backupinfrastructurecontroller "github.com/gardener/gardener/pkg/controller/backupinfrastructure"
	cloudprofilecontroller "github.com/gardener/gardener/pkg/controller/cloudprofile"
	projectcontroller "github.com/gardener/gardener/pkg/controller/project"
	quotacontroller "github.com/gardener/gardener/pkg/controller/quota"
	secretbindingcontroller "github.com/gardener/gardener/pkg/controller/secretbinding"
	seedcontroller "github.com/gardener/gardener/pkg/controller/seed"
	shootcontroller "github.com/gardener/gardener/pkg/controller/shoot"
	"github.com/gardener/gardener/pkg/logger"
	gardenmetrics "github.com/gardener/gardener/pkg/metrics"
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
	config             *componentconfig.ControllerManagerConfiguration
	identity           *gardenv1beta1.Gardener
	gardenNamespace    string
	k8sGardenClient    kubernetes.Client
	k8sGardenInformers gardeninformers.SharedInformerFactory
	k8sInformers       kubeinformers.SharedInformerFactory
	recorder           record.EventRecorder
}

// NewGardenControllerFactory creates a new factory for controllers for the Garden API group.
func NewGardenControllerFactory(k8sGardenClient kubernetes.Client, gardenInformerFactory gardeninformers.SharedInformerFactory, kubeInformerFactory kubeinformers.SharedInformerFactory, config *componentconfig.ControllerManagerConfiguration, identity *gardenv1beta1.Gardener, gardenNamespace string, recorder record.EventRecorder) *GardenControllerFactory {
	return &GardenControllerFactory{
		config:             config,
		identity:           identity,
		gardenNamespace:    gardenNamespace,
		k8sGardenClient:    k8sGardenClient,
		k8sGardenInformers: gardenInformerFactory,
		k8sInformers:       kubeInformerFactory,
		recorder:           recorder,
	}
}

// Run starts all the controllers for the Garden API group. It also performs bootstrapping tasks.
func (f *GardenControllerFactory) Run(ctx context.Context) {
	var (
		cloudProfileInformer         = f.k8sGardenInformers.Garden().V1beta1().CloudProfiles().Informer()
		secretBindingInformer        = f.k8sGardenInformers.Garden().V1beta1().SecretBindings().Informer()
		quotaInformer                = f.k8sGardenInformers.Garden().V1beta1().Quotas().Informer()
		projectInformer              = f.k8sGardenInformers.Garden().V1beta1().Projects().Informer()
		seedInformer                 = f.k8sGardenInformers.Garden().V1beta1().Seeds().Informer()
		shootInformer                = f.k8sGardenInformers.Garden().V1beta1().Shoots().Informer()
		backupInfrastructureInformer = f.k8sGardenInformers.Garden().V1beta1().BackupInfrastructures().Informer()

		namespaceInformer = f.k8sInformers.Core().V1().Namespaces().Informer()
		secretInformer    = f.k8sInformers.Core().V1().Secrets().Informer()
	)

	f.k8sGardenInformers.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), cloudProfileInformer.HasSynced, secretBindingInformer.HasSynced, quotaInformer.HasSynced, projectInformer.HasSynced, seedInformer.HasSynced, shootInformer.HasSynced, backupInfrastructureInformer.HasSynced) {
		panic("Timed out waiting for Garden caches to sync")
	}

	f.k8sInformers.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), namespaceInformer.HasSynced, secretInformer.HasSynced) {
		panic("Timed out waiting for Kube caches to sync")
	}

	secrets, err := garden.ReadGardenSecrets(f.k8sInformers, f.config.ClientConnection.KubeConfigFile == "")
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

	imageVector, err := imagevector.ReadImageVector()
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
		shootController                = shootcontroller.NewShootController(f.k8sGardenClient, f.k8sGardenInformers, f.k8sInformers, f.config, f.identity, f.gardenNamespace, secrets, imageVector, f.recorder)
		seedController                 = seedcontroller.NewSeedController(f.k8sGardenClient, f.k8sGardenInformers, f.k8sInformers, secrets, imageVector, f.config, f.recorder)
		quotaController                = quotacontroller.NewQuotaController(f.k8sGardenClient, f.k8sGardenInformers, f.recorder)
		projectController              = projectcontroller.NewProjectController(f.k8sGardenClient, f.k8sGardenInformers, f.k8sInformers, f.recorder)
		cloudProfileController         = cloudprofilecontroller.NewCloudProfileController(f.k8sGardenClient, f.k8sGardenInformers)
		secretBindingController        = secretbindingcontroller.NewSecretBindingController(f.k8sGardenClient, f.k8sGardenInformers, f.k8sInformers, f.recorder)
		backupInfrastructureController = backupinfrastructurecontroller.NewBackupInfrastructureController(f.k8sGardenClient, f.k8sGardenInformers, f.config, f.identity, f.gardenNamespace, secrets, imageVector, f.recorder)
	)

	// Initialize the Controller metrics collection.
	gardenmetrics.RegisterControllerMetrics(shootController, seedController, quotaController, cloudProfileController, secretBindingController, backupInfrastructureController)

	go shootController.Run(ctx, f.config.Controllers.Shoot.ConcurrentSyncs, f.config.Controllers.ShootCare.ConcurrentSyncs, f.config.Controllers.ShootMaintenance.ConcurrentSyncs, f.config.Controllers.ShootQuota.ConcurrentSyncs)
	go seedController.Run(ctx, f.config.Controllers.Seed.ConcurrentSyncs)
	go quotaController.Run(ctx, f.config.Controllers.Quota.ConcurrentSyncs)
	go projectController.Run(ctx, f.config.Controllers.Project.ConcurrentSyncs)
	go cloudProfileController.Run(ctx, f.config.Controllers.CloudProfile.ConcurrentSyncs)
	go secretBindingController.Run(ctx, f.config.Controllers.SecretBinding.ConcurrentSyncs)
	go backupInfrastructureController.Run(ctx, f.config.Controllers.BackupInfrastructure.ConcurrentSyncs)

	logger.Logger.Infof("Gardener controller manager (version %s) initialized.", version.Version)

	// Shutdown handling
	<-ctx.Done()

	tw := tabwriter.NewWriter(logger.Logger.Writer(), 1, 1, 1, ' ', tabwriter.TabIndent)
	fmt.Fprintf(tw, "BackupInfrastructure:\t%d\n", backupInfrastructureController.RunningWorkers())
	fmt.Fprintf(tw, "CloudProfile:\t%d\n", cloudProfileController.RunningWorkers())
	fmt.Fprintf(tw, "Project:\t%d\n", projectController.RunningWorkers())
	fmt.Fprintf(tw, "Quota:\t%d\n", quotaController.RunningWorkers())
	fmt.Fprintf(tw, "SecretBinding:\t%d\n", secretBindingController.RunningWorkers())
	fmt.Fprintf(tw, "Seed:\t%d\n", seedController.RunningWorkers())
	fmt.Fprintf(tw, "Shoot:\t%d\n", shootController.RunningWorkers())

	logger.Logger.Infof("I have received a stop signal and will no longer watch events of the Garden API group.")
	logger.Logger.Infof("NUMBER OF REMAINING WORKERS:")
	logger.Logger.Infof("============================")
	tw.Flush()
	logger.Logger.Infof("============================")
	logger.Logger.Infof("Bye Bye!")
}
