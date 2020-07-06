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
	"sync"
	"time"

	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/controllermanager"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// FinalizerName is the finalizer used by this controller.
const FinalizerName = "core.gardener.cloud/controllerregistration"

// Controller controls ControllerRegistration.
type Controller struct {
	clientMap clientmap.ClientMap

	controllerRegistrationControl     ControlInterface
	controllerRegistrationSeedControl RegistrationSeedControlInterface
	seedControl                       SeedControlInterface

	backupBucketSynced           cache.InformerSynced
	backupEntrySynced            cache.InformerSynced
	controllerInstallationSynced cache.InformerSynced
	controllerRegistrationSynced cache.InformerSynced
	seedSynced                   cache.InformerSynced
	shootSynced                  cache.InformerSynced

	backupBucketLister           gardencorelisters.BackupBucketLister
	controllerRegistrationLister gardencorelisters.ControllerRegistrationLister
	seedLister                   gardencorelisters.SeedLister

	controllerRegistrationQueue     workqueue.RateLimitingInterface
	controllerRegistrationSeedQueue workqueue.RateLimitingInterface
	seedQueue                       workqueue.RateLimitingInterface

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewController instantiates a new ControllerRegistration controller.
func NewController(
	clientMap clientmap.ClientMap,
	gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory,
	secrets map[string]*corev1.Secret,
) *Controller {
	var (
		gardenCoreInformer = gardenCoreInformerFactory.Core().V1beta1()

		backupBucketInformer = gardenCoreInformer.BackupBuckets()
		backupBucketLister   = backupBucketInformer.Lister()

		backupEntryInformer = gardenCoreInformer.BackupEntries()

		controllerRegistrationInformer = gardenCoreInformer.ControllerRegistrations()
		controllerRegistrationLister   = controllerRegistrationInformer.Lister()

		controllerInstallationInformer = gardenCoreInformer.ControllerInstallations()
		controllerInstallationLister   = controllerInstallationInformer.Lister()

		seedInformer = gardenCoreInformer.Seeds()
		seedLister   = seedInformer.Lister()

		shootInformer = gardenCoreInformer.Shoots()

		controllerRegistrationQueue     = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "controllerregistration")
		controllerRegistrationSeedQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "controllerregistration-seed")
		seedQueue                       = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "seed")
	)

	controller := &Controller{
		clientMap: clientMap,

		controllerRegistrationControl:     NewDefaultControllerRegistrationControl(clientMap, controllerInstallationLister),
		controllerRegistrationSeedControl: NewDefaultControllerRegistrationSeedControl(clientMap, secrets, backupBucketLister, controllerInstallationLister, controllerRegistrationLister, seedLister),
		seedControl:                       NewDefaultSeedControl(clientMap, controllerInstallationLister),

		backupBucketLister:           backupBucketLister,
		controllerRegistrationLister: controllerRegistrationLister,
		seedLister:                   seedLister,

		controllerRegistrationQueue:     controllerRegistrationQueue,
		controllerRegistrationSeedQueue: controllerRegistrationSeedQueue,
		seedQueue:                       seedQueue,

		workerCh: make(chan int),
	}

	backupBucketInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.backupBucketAdd,
		UpdateFunc: controller.backupBucketUpdate,
		DeleteFunc: controller.backupBucketDelete,
	})
	controller.backupBucketSynced = backupBucketInformer.Informer().HasSynced

	backupEntryInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.backupEntryAdd,
		UpdateFunc: controller.backupEntryUpdate,
		DeleteFunc: controller.backupEntryDelete,
	})
	controller.backupEntrySynced = backupEntryInformer.Informer().HasSynced

	controllerRegistrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.controllerRegistrationAdd,
		UpdateFunc: controller.controllerRegistrationUpdate,
		DeleteFunc: controller.controllerRegistrationDelete,
	})
	controller.controllerRegistrationSynced = controllerRegistrationInformer.Informer().HasSynced

	controllerInstallationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.controllerInstallationAdd,
		UpdateFunc: controller.controllerInstallationUpdate,
	})
	controller.controllerInstallationSynced = controllerInstallationInformer.Informer().HasSynced

	seedInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { controller.seedAdd(obj, true) },
		UpdateFunc: controller.seedUpdate,
		DeleteFunc: controller.seedDelete,
	})
	controller.seedSynced = seedInformer.Informer().HasSynced

	shootInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.shootAdd,
		UpdateFunc: controller.shootUpdate,
		DeleteFunc: controller.shootDelete,
	})
	controller.shootSynced = shootInformer.Informer().HasSynced

	return controller
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.backupBucketSynced, c.backupEntrySynced, c.controllerRegistrationSynced, c.controllerInstallationSynced, c.seedSynced, c.shootSynced) {
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
		controllerutils.DeprecatedCreateWorker(ctx, c.controllerRegistrationQueue, "ControllerRegistration", c.reconcileControllerRegistrationKey, &waitGroup, c.workerCh)
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
