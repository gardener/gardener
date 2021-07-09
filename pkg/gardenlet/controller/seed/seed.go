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

package seed

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/gardenlet/controller/lease"
	"github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// Controller controls Seeds.
type Controller struct {
	clientMap              clientmap.ClientMap
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory

	config        *config.GardenletConfiguration
	healthManager healthz.Manager
	recorder      record.EventRecorder

	control               ControlInterface
	extensionCheckControl ExtensionCheckControlInterface
	seedLeaseControl      lease.Controller

	seedLister gardencorelisters.SeedLister

	hasSyncedFuncs []cache.InformerSynced

	seedQueue               workqueue.RateLimitingInterface
	seedLeaseQueue          workqueue.RateLimitingInterface
	seedExtensionCheckQueue workqueue.RateLimitingInterface

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewSeedController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <seedInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewSeedController(
	clientMap clientmap.ClientMap,
	gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory,
	healthManager healthz.Manager,
	imageVector imagevector.ImageVector,
	componentImageVectors imagevector.ComponentImageVectors,
	identity *gardencorev1beta1.Gardener,
	config *config.GardenletConfiguration,
	recorder record.EventRecorder,
) *Controller {
	var (
		gardenCoreV1beta1Informer = gardenCoreInformerFactory.Core().V1beta1()

		controllerInstallationInformer = gardenCoreV1beta1Informer.ControllerInstallations()
		seedInformer                   = gardenCoreV1beta1Informer.Seeds()

		controllerInstallationLister = controllerInstallationInformer.Lister()
		seedLister                   = seedInformer.Lister()
	)

	seedController := &Controller{
		clientMap:               clientMap,
		k8sGardenCoreInformers:  gardenCoreInformerFactory,
		config:                  config,
		healthManager:           healthManager,
		recorder:                recorder,
		control:                 NewDefaultControl(clientMap, gardenCoreInformerFactory, imageVector, componentImageVectors, identity, recorder, config, seedLister),
		extensionCheckControl:   NewDefaultExtensionCheckControl(clientMap, controllerInstallationLister, metav1.Now),
		seedLeaseControl:        lease.NewLeaseController(time.Now, clientMap, LeaseResyncSeconds, gardencorev1beta1.GardenerSeedLeaseNamespace),
		seedLister:              seedLister,
		seedQueue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "seed"),
		seedLeaseQueue:          workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(time.Millisecond, 2*time.Second), "seed-lease"),
		seedExtensionCheckQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "seed-extension-check"),
		workerCh:                make(chan int),
	}

	seedInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.SeedFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig)),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    seedController.seedAdd,
			UpdateFunc: seedController.seedUpdate,
			DeleteFunc: seedController.seedDelete,
		},
	})

	seedInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.SeedFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig)),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: seedController.seedLeaseAdd,
		},
	})

	controllerInstallationInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.ControllerInstallationFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig)),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    seedController.controllerInstallationOfSeedAdd,
			UpdateFunc: seedController.controllerInstallationOfSeedUpdate,
			DeleteFunc: seedController.controllerInstallationOfSeedDelete,
		},
	})

	seedController.hasSyncedFuncs = []cache.InformerSynced{
		seedInformer.Informer().HasSynced,
		controllerInstallationInformer.Informer().HasSynced,
	}

	return seedController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running Seed workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("Seed controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.DeprecatedCreateWorker(ctx, c.seedQueue, "Seed", c.reconcileSeedKey, &waitGroup, c.workerCh)
		controllerutils.DeprecatedCreateWorker(ctx, c.seedLeaseQueue, "Seed Lease", c.reconcileSeedLeaseKey, &waitGroup, c.workerCh)
		controllerutils.DeprecatedCreateWorker(ctx, c.seedExtensionCheckQueue, "Seed Extension Check", c.reconcileSeedExtensionCheckKey, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.seedQueue.ShutDown()
	c.seedLeaseQueue.ShutDown()
	c.seedExtensionCheckQueue.ShutDown()

	for {
		if c.seedQueue.Len() == 0 && c.seedLeaseQueue.Len() == 0 && c.seedExtensionCheckQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running Seed worker and no items left in the queues. Terminated Seed controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d Seed worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.seedQueue.Len()+c.seedLeaseQueue.Len()+c.seedExtensionCheckQueue.Len())
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
	metric, err := prometheus.NewConstMetric(gardenlet.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "seed")
	if err != nil {
		gardenlet.ScrapeFailures.With(prometheus.Labels{"kind": "seed-controller"}).Inc()
		return
	}
	ch <- metric
}
