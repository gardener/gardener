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

package controllerregistration

import (
	"context"
	"sync"
	"time"

	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1alpha1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	controllerutils "github.com/gardener/gardener/pkg/controllermanager/controller/utils"
	gardenmetrics "github.com/gardener/gardener/pkg/controllermanager/metrics"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// FinalizerName is the name of the ControllerRegistration finalizer.
const FinalizerName = "core.gardener.cloud/controllerregistration"

// Controller controls ControllerRegistration.
type Controller struct {
	k8sGardenClient        kubernetes.Interface
	k8sGardenInformers     gardeninformers.SharedInformerFactory
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory

	config *config.ControllerManagerConfiguration

	seedControl                   SeedControlInterface
	controllerRegistrationControl ControlInterface

	recorder record.EventRecorder

	seedQueue  workqueue.RateLimitingInterface
	seedLister gardenlisters.SeedLister
	seedSynced cache.InformerSynced

	controllerRegistrationQueue  workqueue.RateLimitingInterface
	controllerRegistrationLister gardencorelisters.ControllerRegistrationLister
	controllerRegistrationSynced cache.InformerSynced

	controllerInstallationSynced cache.InformerSynced

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewController instantiates a new ControllerRegistration controller.
func NewController(k8sGardenClient kubernetes.Interface, gardenInformerFactory gardeninformers.SharedInformerFactory, gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory, config *config.ControllerManagerConfiguration, recorder record.EventRecorder) *Controller {
	var (
		gardenInformer     = gardenInformerFactory.Garden().V1beta1()
		gardenCoreInformer = gardenCoreInformerFactory.Core().V1alpha1()

		seedInformer = gardenInformer.Seeds()
		seedLister   = seedInformer.Lister()
		seedQueue    = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "seed")

		controllerRegistrationInformer = gardenCoreInformer.ControllerRegistrations()
		controllerRegistrationLister   = controllerRegistrationInformer.Lister()
		controllerRegistrationQueue    = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "controllerregistration")

		controllerInstallationInformer = gardenCoreInformer.ControllerInstallations()
		controllerInstallationLister   = controllerInstallationInformer.Lister()
	)

	controller := &Controller{
		k8sGardenClient:               k8sGardenClient,
		k8sGardenInformers:            gardenInformerFactory,
		k8sGardenCoreInformers:        gardenCoreInformerFactory,
		seedControl:                   NewDefaultSeedControl(k8sGardenClient, gardenInformerFactory, gardenCoreInformerFactory, recorder, config, controllerRegistrationLister, controllerInstallationLister, controllerRegistrationQueue),
		controllerRegistrationControl: NewDefaultControllerRegistrationControl(k8sGardenClient, gardenInformerFactory, gardenCoreInformerFactory, recorder, config, seedLister, controllerRegistrationLister, controllerInstallationLister),
		config:                        config,
		recorder:                      recorder,

		seedLister: seedLister,
		seedQueue:  seedQueue,

		controllerRegistrationLister: controllerRegistrationLister,
		controllerRegistrationQueue:  controllerRegistrationQueue,

		workerCh: make(chan int),
	}

	seedInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.seedAdd,
		UpdateFunc: controller.seedUpdate,
		DeleteFunc: controller.seedDelete,
	})
	controller.seedSynced = seedInformer.Informer().HasSynced

	controllerRegistrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.controllerRegistrationAdd,
		UpdateFunc: controller.controllerRegistrationUpdate,
		DeleteFunc: controller.controllerRegistrationDelete,
	})
	controller.controllerRegistrationSynced = controllerRegistrationInformer.Informer().HasSynced

	controller.controllerInstallationSynced = controllerInstallationInformer.Informer().HasSynced

	return controller
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.seedSynced, c.controllerRegistrationSynced, c.controllerInstallationSynced) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	go func() {
		for {
			select {
			case res := <-c.workerCh:
				c.numberOfRunningWorkers += res
				logger.Logger.Debugf("Current number of running ControllerRegistration workers is %d", c.numberOfRunningWorkers)
			}
		}
	}()

	logger.Logger.Info("ControllerRegistration controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.seedQueue, "Seed", c.reconcileSeedKey, &waitGroup, c.workerCh)
		controllerutils.CreateWorker(ctx, c.controllerRegistrationQueue, "ControllerRegistration", c.reconcileControllerRegistrationKey, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.seedQueue.ShutDown()
	c.controllerRegistrationQueue.ShutDown()

	for {
		if c.controllerRegistrationQueue.Len() == 0 && c.seedQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running ControllerRegistration worker and no items left in the queues. Terminated ControllerRegistration controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d ControllerRegistration worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.controllerRegistrationQueue.Len()+c.seedQueue.Len())
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
	metric, err := prometheus.NewConstMetric(gardenmetrics.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "controllerregistration")
	if err != nil {
		gardenmetrics.ScrapeFailures.With(prometheus.Labels{"kind": "controllerregistration-controller"}).Inc()
		return
	}
	ch <- metric
}
