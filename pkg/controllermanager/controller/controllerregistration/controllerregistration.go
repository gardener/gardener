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
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllermanager"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// FinalizerName is the finalizer used by this controller.
const FinalizerName = "core.gardener.cloud/controllerregistration"

// Controller controls ControllerRegistration.
type Controller struct {
	controllerRegistrationReconciler  reconcile.Reconciler
	controllerRegistrationSeedControl RegistrationSeedControlInterface
	seedControl                       SeedControlInterface
	hasSyncedFuncs                    []cache.InformerSynced

	seedLister gardencorelisters.SeedLister

	controllerRegistrationQueue     workqueue.RateLimitingInterface
	controllerRegistrationSeedQueue workqueue.RateLimitingInterface
	seedQueue                       workqueue.RateLimitingInterface
	workerCh                        chan int
	numberOfRunningWorkers          int
}

// NewController instantiates a new ControllerRegistration controller.
func NewController(
	ctx context.Context,
	clientMap clientmap.ClientMap,
	gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
) (
	*Controller,
	error,
) {
	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, err
	}

	backupBucketInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.BackupBucket{})
	if err != nil {
		return nil, fmt.Errorf("failed to get BackupBucket Informer: %w", err)
	}

	controllerRegistrationInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.ControllerRegistration{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ControllerRegistration Informer: %w", err)
	}

	var (
		gardenCoreInformer = gardenCoreInformerFactory.Core().V1beta1()
		k8sCoreInformer    = kubeInformerFactory.Core().V1()

		backupEntryInformer = gardenCoreInformer.BackupEntries()

		controllerInstallationInformer = gardenCoreInformer.ControllerInstallations()
		controllerInstallationLister   = controllerInstallationInformer.Lister()

		seedInformer = gardenCoreInformer.Seeds()
		seedLister   = seedInformer.Lister()

		shootInformer = gardenCoreInformer.Shoots()

		secretInformer = k8sCoreInformer.Secrets()
		secretLister   = secretInformer.Lister()

		controllerRegistrationQueue     = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "controllerregistration")
		controllerRegistrationSeedQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "controllerregistration-seed")
		seedQueue                       = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "seed")
	)

	controller := &Controller{
		controllerRegistrationReconciler:  NewControllerRegistrationReconciler(logger.Logger, gardenClient.Client()),
		controllerRegistrationSeedControl: NewDefaultControllerRegistrationSeedControl(gardenClient, secretLister, seedLister),
		seedControl:                       NewDefaultSeedControl(clientMap, controllerInstallationLister),

		seedLister: seedLister,

		controllerRegistrationQueue:     controllerRegistrationQueue,
		controllerRegistrationSeedQueue: controllerRegistrationSeedQueue,
		seedQueue:                       seedQueue,

		workerCh: make(chan int),
	}

	backupBucketInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.backupBucketAdd,
		UpdateFunc: controller.backupBucketUpdate,
		DeleteFunc: controller.backupBucketDelete,
	})

	backupEntryInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.backupEntryAdd,
		UpdateFunc: controller.backupEntryUpdate,
		DeleteFunc: controller.backupEntryDelete,
	})

	controllerRegistrationInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.controllerRegistrationAdd,
		UpdateFunc: controller.controllerRegistrationUpdate,
		DeleteFunc: controller.controllerRegistrationDelete,
	})

	controllerInstallationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.controllerInstallationAdd,
		UpdateFunc: controller.controllerInstallationUpdate,
	})

	seedInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { controller.seedAdd(obj, true) },
		UpdateFunc: controller.seedUpdate,
		DeleteFunc: controller.seedDelete,
	})

	shootInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.shootAdd,
		UpdateFunc: controller.shootUpdate,
		DeleteFunc: controller.shootDelete,
	})

	controller.hasSyncedFuncs = append(controller.hasSyncedFuncs,
		backupBucketInformer.HasSynced,
		backupEntryInformer.Informer().HasSynced,
		controllerRegistrationInformer.HasSynced,
		controllerInstallationInformer.Informer().HasSynced,
		seedInformer.Informer().HasSynced,
		shootInformer.Informer().HasSynced,
	)

	return controller, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running ControllerRegistration workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("ControllerRegistration controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.controllerRegistrationQueue, "ControllerRegistration", c.controllerRegistrationReconciler, &waitGroup, c.workerCh)
		controllerutils.DeprecatedCreateWorker(ctx, c.controllerRegistrationSeedQueue, "ControllerRegistration-Seed", c.reconcileControllerRegistrationSeedKey, &waitGroup, c.workerCh)
		controllerutils.DeprecatedCreateWorker(ctx, c.seedQueue, "Seed", c.reconcileSeedKey, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.controllerRegistrationQueue.ShutDown()
	c.controllerRegistrationSeedQueue.ShutDown()
	c.seedQueue.ShutDown()

	for {
		if c.controllerRegistrationQueue.Len() == 0 && c.seedQueue.Len() == 0 && c.controllerRegistrationSeedQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running ControllerRegistration worker and no items left in the queues. Terminated ControllerRegistration controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d ControllerRegistration worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.controllerRegistrationQueue.Len()+c.seedQueue.Len()+c.controllerRegistrationSeedQueue.Len())
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}

// RunningWorkers returns the number of running workers.
func (c *Controller) RunningWorkers() int {
	return c.numberOfRunningWorkers
}

// CollectMetrics implements gardenmetrics.ControllerMetricsCollector interface
func (c *Controller) CollectMetrics(ch chan<- prometheus.Metric) {
	metric, err := prometheus.NewConstMetric(controllermanager.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "controllerregistration")
	if err != nil {
		controllermanager.ScrapeFailures.With(prometheus.Labels{"kind": "controllerregistration-controller"}).Inc()
		return
	}
	ch <- metric
}
