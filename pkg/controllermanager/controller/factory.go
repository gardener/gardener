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
	"errors"
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllermanager"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	csrcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/certificatesigningrequest"
	cloudprofilecontroller "github.com/gardener/gardener/pkg/controllermanager/controller/cloudprofile"
	controllerregistrationcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration"
	eventcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/event"
	plantcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/plant"
	projectcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/project"
	quotacontroller "github.com/gardener/gardener/pkg/controllermanager/controller/quota"
	secretbindingcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/secretbinding"
	seedcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/seed"
	shootcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/shoot"
	gardenmetrics "github.com/gardener/gardener/pkg/controllerutils/metrics"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/version"

	"k8s.io/apimachinery/pkg/util/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

// GardenControllerFactory contains information relevant to controllers for the Garden API group.
type GardenControllerFactory struct {
	cfg                    *config.ControllerManagerConfiguration
	clientMap              clientmap.ClientMap
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory
	k8sInformers           kubeinformers.SharedInformerFactory
	recorder               record.EventRecorder
}

// NewGardenControllerFactory creates a new factory for controllers for the Garden API group.
func NewGardenControllerFactory(clientMap clientmap.ClientMap, gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory, kubeInformerFactory kubeinformers.SharedInformerFactory, cfg *config.ControllerManagerConfiguration, recorder record.EventRecorder) *GardenControllerFactory {
	return &GardenControllerFactory{
		cfg:                    cfg,
		clientMap:              clientMap,
		k8sGardenCoreInformers: gardenCoreInformerFactory,
		k8sInformers:           kubeInformerFactory,
		recorder:               recorder,
	}
}

// Run starts all the controllers for the Garden API group. It also performs bootstrapping tasks.
func (f *GardenControllerFactory) Run(ctx context.Context) error {
	var (
		// Garden core informers
		backupBucketInformer           = f.k8sGardenCoreInformers.Core().V1beta1().BackupBuckets().Informer()
		backupEntryInformer            = f.k8sGardenCoreInformers.Core().V1beta1().BackupEntries().Informer()
		cloudProfileInformer           = f.k8sGardenCoreInformers.Core().V1beta1().CloudProfiles().Informer()
		controllerRegistrationInformer = f.k8sGardenCoreInformers.Core().V1beta1().ControllerRegistrations().Informer()
		controllerInstallationInformer = f.k8sGardenCoreInformers.Core().V1beta1().ControllerInstallations().Informer()
		quotaInformer                  = f.k8sGardenCoreInformers.Core().V1beta1().Quotas().Informer()
		plantInformer                  = f.k8sGardenCoreInformers.Core().V1beta1().Plants().Informer()
		projectInformer                = f.k8sGardenCoreInformers.Core().V1beta1().Projects().Informer()
		secretBindingInformer          = f.k8sGardenCoreInformers.Core().V1beta1().SecretBindings().Informer()
		seedInformer                   = f.k8sGardenCoreInformers.Core().V1beta1().Seeds().Informer()
		shootInformer                  = f.k8sGardenCoreInformers.Core().V1beta1().Shoots().Informer()
		// Kubernetes core informers
		configMapInformer   = f.k8sInformers.Core().V1().ConfigMaps().Informer()
		csrInformer         = f.k8sInformers.Certificates().V1beta1().CertificateSigningRequests().Informer()
		namespaceInformer   = f.k8sInformers.Core().V1().Namespaces().Informer()
		secretInformer      = f.k8sInformers.Core().V1().Secrets().Informer()
		roleBindingInformer = f.k8sInformers.Rbac().V1().RoleBindings().Informer()
		leaseInformer       = f.k8sInformers.Coordination().V1().Leases().Informer()
	)

	if err := f.clientMap.Start(ctx.Done()); err != nil {
		panic(fmt.Errorf("failed to start ClientMap: %+v", err))
	}

	k8sGardenClient, err := f.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		panic(fmt.Errorf("failed to get garden client: %+v", err))
	}

	f.k8sGardenCoreInformers.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), backupBucketInformer.HasSynced, backupEntryInformer.HasSynced, controllerRegistrationInformer.HasSynced, controllerInstallationInformer.HasSynced, plantInformer.HasSynced, cloudProfileInformer.HasSynced, secretBindingInformer.HasSynced, quotaInformer.HasSynced, projectInformer.HasSynced, seedInformer.HasSynced, shootInformer.HasSynced) {
		return errors.New("Timed out waiting for Garden core caches to sync")
	}

	f.k8sInformers.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), configMapInformer.HasSynced, csrInformer.HasSynced, namespaceInformer.HasSynced, secretInformer.HasSynced, roleBindingInformer.HasSynced, leaseInformer.HasSynced) {
		return errors.New("Timed out waiting for Kube caches to sync")
	}

	secrets, err := garden.ReadGardenSecrets(f.k8sInformers, f.k8sGardenCoreInformers)
	runtime.Must(err)

	runtime.Must(garden.BootstrapCluster(k8sGardenClient, v1beta1constants.GardenNamespace, secrets))
	logger.Logger.Info("Successfully bootstrapped the Garden cluster.")

	// Initialize the workqueue metrics collection.
	gardenmetrics.RegisterWorkqueMetrics()

	var (
		cloudProfileController           = cloudprofilecontroller.NewCloudProfileController(f.clientMap, f.k8sGardenCoreInformers, f.recorder)
		controllerRegistrationController = controllerregistrationcontroller.NewController(f.clientMap, f.k8sGardenCoreInformers, secrets)
		csrController                    = csrcontroller.NewCSRController(f.clientMap, f.k8sInformers, f.recorder)
		quotaController                  = quotacontroller.NewQuotaController(f.clientMap, f.k8sGardenCoreInformers, f.recorder)
		plantController                  = plantcontroller.NewController(f.clientMap, f.k8sGardenCoreInformers, f.k8sInformers, f.cfg, f.recorder)
		projectController                = projectcontroller.NewProjectController(f.clientMap, f.k8sGardenCoreInformers, f.k8sInformers, f.cfg, f.recorder)
		secretBindingController          = secretbindingcontroller.NewSecretBindingController(f.clientMap, f.k8sGardenCoreInformers, f.k8sInformers, f.recorder)
		seedController                   = seedcontroller.NewSeedController(f.clientMap, f.k8sGardenCoreInformers, f.k8sInformers, f.cfg, f.recorder)
		eventController                  = eventcontroller.NewController(f.clientMap, f.cfg.Controllers.Event)
	)

	shootController, err := shootcontroller.NewShootController(f.clientMap, f.k8sGardenCoreInformers, f.k8sInformers, f.cfg, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing Shoot controller: %w", err)
	}

	// Initialize the Controller metrics collection.
	gardenmetrics.RegisterControllerMetrics(
		controllermanager.ControllerWorkerSum,
		controllermanager.ScrapeFailures,
		controllerRegistrationController,
		cloudProfileController,
		csrController,
		quotaController,
		plantController,
		projectController,
		secretBindingController,
		seedController,
		shootController,
		eventController,
	)

	go cloudProfileController.Run(ctx, f.cfg.Controllers.CloudProfile.ConcurrentSyncs)
	go controllerRegistrationController.Run(ctx, f.cfg.Controllers.ControllerRegistration.ConcurrentSyncs)
	go csrController.Run(ctx, 1)
	go plantController.Run(ctx, f.cfg.Controllers.Plant.ConcurrentSyncs)
	go projectController.Run(ctx, f.cfg.Controllers.Project.ConcurrentSyncs)
	go quotaController.Run(ctx, f.cfg.Controllers.Quota.ConcurrentSyncs)
	go secretBindingController.Run(ctx, f.cfg.Controllers.SecretBinding.ConcurrentSyncs)
	go seedController.Run(ctx, f.cfg.Controllers.Seed.ConcurrentSyncs)
	go shootController.Run(ctx, f.cfg.Controllers.ShootMaintenance.ConcurrentSyncs, f.cfg.Controllers.ShootQuota.ConcurrentSyncs, f.cfg.Controllers.ShootHibernation.ConcurrentSyncs, f.cfg.Controllers.ShootReference.ConcurrentSyncs)
	go eventController.Run(ctx)

	logger.Logger.Infof("Gardener controller manager (version %s) initialized.", version.Get().GitVersion)

	// Shutdown handling
	<-ctx.Done()

	logger.Logger.Infof("I have received a stop signal and will no longer watch resources.")
	logger.Logger.Infof("Bye Bye!")

	return nil
}
