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

package managedseed

import (
	"context"
	"sync"
	"time"

	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	seedmanagementinformers "github.com/gardener/gardener/pkg/client/seedmanagement/informers/externalversions"
	seedmanagementlisters "github.com/gardener/gardener/pkg/client/seedmanagement/listers/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/imagevector"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller controls Shoots.
type Controller struct {
	config *config.GardenletConfiguration

	reconciler reconcile.Reconciler

	managedSeedLister seedmanagementlisters.ManagedSeedLister
	managedSeedQueue  workqueue.RateLimitingInterface

	managedSeedSynced cache.InformerSynced
	seedSynced        cache.InformerSynced
	shootSynced       cache.InformerSynced

	numberOfRunningWorkers int
	workerCh               chan int
}

// NewManagedSeedController creates a new Gardener controller for ManagedSeeds.
func NewManagedSeedController(clientMap clientmap.ClientMap, seedManagementInformers seedmanagementinformers.SharedInformerFactory, gardenCoreInformers gardencoreinformers.SharedInformerFactory, config *config.GardenletConfiguration, imageVector imagevector.ImageVector, recorder record.EventRecorder) *Controller {
	var (
		seedManagementV1alpha1Informer = seedManagementInformers.Seedmanagement().V1alpha1()
		gardenCoreV1beta1Informer      = gardenCoreInformers.Core().V1beta1()

		managedSeedInformer = seedManagementV1alpha1Informer.ManagedSeeds()
		managedSeedLister   = managedSeedInformer.Lister()

		seedInformer = gardenCoreV1beta1Informer.Seeds()
		seedLister   = seedInformer.Lister()

		shootInformer = gardenCoreV1beta1Informer.Shoots()
		shootLister   = shootInformer.Lister()
	)

	controller := &Controller{
		config: config,

		reconciler: newReconciler(context.TODO(), clientMap, config, imageVector, recorder, logger.Logger),

		managedSeedLister: managedSeedLister,
		managedSeedQueue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "seed-managedSeedistration"),

		workerCh: make(chan int),
	}

	managedSeedInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.ManagedSeedFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig), seedLister, shootLister, config.SeedSelector),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				controller.managedSeedAdd(obj, false)
			},
			UpdateFunc: controller.managedSeedUpdate,
			DeleteFunc: controller.managedSeedDelete,
		},
	})

	controller.managedSeedSynced = managedSeedInformer.Informer().HasSynced
	controller.seedSynced = seedInformer.Informer().HasSynced
	controller.shootSynced = shootInformer.Informer().HasSynced

	return controller
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.managedSeedSynced, c.seedSynced, c.shootSynced) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running ManagedSeed workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("ManagedSeed controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.managedSeedQueue, "ManagedSeed", c.reconciler, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.managedSeedQueue.ShutDown()

	for {
		if c.managedSeedQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running ManagedSeed worker and no items left in the queues. Terminated ManagedSeed controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d ManagedSeed worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.managedSeedQueue.Len())
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
	metric, err := prometheus.NewConstMetric(gardenlet.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "managedSeed")
	if err != nil {
		gardenlet.ScrapeFailures.With(prometheus.Labels{"kind": "managedSeed-controller"}).Inc()
		return
	}
	ch <- metric
}
