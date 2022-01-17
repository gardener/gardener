// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controllerregistration

import (
	"context"
	"fmt"
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// FinalizerName is the finalizer used by this controller.
	FinalizerName = "core.gardener.cloud/controllerregistration"

	// ControllerName is the name of this controller.
	ControllerName = "controllerregistration"
)

// Controller implements the logic behind ControllerRegistrations. It creates and deletes ControllerInstallations for
// ControllerRegistrations for the Seeds where they are needed or not.
type Controller struct {
	gardenClient client.Client
	log          logr.Logger

	// main reconciler of this controller: deploys and deletes ControllerInstallations for Seeds according to the Shoots,
	// etc. scheduled to a given Seed
	seedReconciler reconcile.Reconciler
	// manages finalizer on ControllerRegistrations depending on referencing ControllerInstallations
	controllerRegistrationFinalizerReconciler reconcile.Reconciler
	// manages finalizer on Seeds depending on referencing ControllerInstallations
	seedFinalizerReconciler reconcile.Reconciler

	seedQueue                            workqueue.RateLimitingInterface
	controllerRegistrationFinalizerQueue workqueue.RateLimitingInterface
	seedFinalizerQueue                   workqueue.RateLimitingInterface

	hasSyncedFuncs         []cache.InformerSynced
	workerCh               chan int
	numberOfRunningWorkers int
}

// NewController instantiates a new ControllerRegistration controller.
func NewController(ctx context.Context, log logr.Logger, clientMap clientmap.ClientMap) (*Controller, error) {
	log = log.WithName(ControllerName)

	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, err
	}

	backupBucketInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.BackupBucket{})
	if err != nil {
		return nil, fmt.Errorf("failed to get BackupBucket Informer: %w", err)
	}
	backupEntryInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.BackupEntry{})
	if err != nil {
		return nil, fmt.Errorf("failed to get BackupEntry Informer: %w", err)
	}
	controllerDeploymentInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.ControllerDeployment{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ControllerDeployment Informer: %w", err)
	}
	controllerInstallationInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.ControllerInstallation{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ControllerInstallation Informer: %w", err)
	}
	controllerRegistrationInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.ControllerRegistration{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ControllerRegistration Informer: %w", err)
	}
	seedInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Seed{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Seed Informer: %w", err)
	}
	shootInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Shoot{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Shoot Informer: %w", err)
	}

	controller := &Controller{
		gardenClient: gardenClient.Client(),
		log:          log,

		seedReconciler: NewSeedReconciler(gardenClient),
		controllerRegistrationFinalizerReconciler: NewControllerRegistrationFinalizerReconciler(gardenClient.Client()),
		seedFinalizerReconciler:                   NewSeedFinalizerReconciler(gardenClient.Client()),

		seedQueue:                            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "seed"),
		controllerRegistrationFinalizerQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "controllerregistration-finalizer"),
		seedFinalizerQueue:                   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "seed-finalizer"),
		workerCh:                             make(chan int),
	}

	backupBucketInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.backupBucketAdd,
		UpdateFunc: controller.backupBucketUpdate,
		DeleteFunc: controller.backupBucketDelete,
	})

	backupEntryInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.backupEntryAdd,
		UpdateFunc: controller.backupEntryUpdate,
		DeleteFunc: controller.backupEntryDelete,
	})

	controllerRegistrationInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { controller.controllerRegistrationAdd(ctx, obj) },
		UpdateFunc: func(oldObj, newObj interface{}) { controller.controllerRegistrationUpdate(ctx, oldObj, newObj) },
		DeleteFunc: controller.controllerRegistrationDelete,
	})

	controllerDeploymentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { controller.controllerDeploymentAdd(ctx, obj) },
		UpdateFunc: func(oldObj, newObj interface{}) { controller.controllerDeploymentUpdate(ctx, oldObj, newObj) },
	})

	controllerInstallationInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.controllerInstallationAdd,
		UpdateFunc: controller.controllerInstallationUpdate,
	})

	seedInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { controller.seedAdd(obj, true) },
		UpdateFunc: controller.seedUpdate,
		DeleteFunc: controller.seedDelete,
	})

	shootInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.shootAdd,
		UpdateFunc: controller.shootUpdate,
		DeleteFunc: controller.shootDelete,
	})

	controller.hasSyncedFuncs = append(controller.hasSyncedFuncs,
		backupBucketInformer.HasSynced,
		backupEntryInformer.HasSynced,
		controllerRegistrationInformer.HasSynced,
		controllerDeploymentInformer.HasSynced,
		controllerInstallationInformer.HasSynced,
		seedInformer.HasSynced,
		shootInformer.HasSynced,
	)

	return controller, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		c.log.Error(wait.ErrWaitTimeout, "Timed out waiting for caches to sync")
		return
	}

	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
		}
	}()

	c.log.Info("ControllerRegistration controller initialized")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.seedQueue, "ControllerRegistration-Seed", c.seedReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(seedReconcilerName)))
		controllerutils.CreateWorker(ctx, c.controllerRegistrationFinalizerQueue, "ControllerRegistration-Finalizer", c.controllerRegistrationFinalizerReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(controllerRegistrationFinalizerReconcilerName)))
		controllerutils.CreateWorker(ctx, c.seedFinalizerQueue, "Seed-Finalizer", c.seedFinalizerReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(seedFinalizerReconcilerName)))
	}

	// Shutdown handling
	<-ctx.Done()
	c.seedQueue.ShutDown()
	c.controllerRegistrationFinalizerQueue.ShutDown()
	c.seedFinalizerQueue.ShutDown()

	for {
		if c.controllerRegistrationFinalizerQueue.Len() == 0 && c.seedFinalizerQueue.Len() == 0 && c.seedQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			c.log.V(1).Info("No running ControllerRegistration worker and no items left in the queues. Terminating ControllerRegistration controller...")
			break
		}
		c.log.V(1).Info("Waiting for ControllerRegistration workers to finish...", "numberOfRunningWorkers", c.numberOfRunningWorkers, "queueLength", c.controllerRegistrationFinalizerQueue.Len()+c.seedFinalizerQueue.Len()+c.seedQueue.Len())
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}

func (c *Controller) enqueueAllSeeds(ctx context.Context) {
	seedList := &metav1.PartialObjectMetadataList{}
	seedList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("SeedList"))
	if err := c.gardenClient.List(ctx, seedList); err != nil {
		return
	}

	for _, seed := range seedList.Items {
		c.seedQueue.Add(seed.Name)
	}
}
