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

package plant

import (
	"context"
	"sync"
	"time"

	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"

	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	controllerutils "github.com/gardener/gardener/pkg/controllermanager/controller/utils"
	gardenmetrics "github.com/gardener/gardener/pkg/controllermanager/metrics"
	"github.com/gardener/gardener/pkg/logger"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"github.com/prometheus/client_golang/prometheus"
)

// FinalizerName is the name of the Plant finalizer.
const FinalizerName = "core.gardener.cloud/plant"

// Controller controls Plant.
type Controller struct {
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory
	k8sInformers           kubeinformers.SharedInformerFactory
	config                 *config.ControllerManagerConfiguration

	recorder record.EventRecorder

	secretLister kubecorev1listers.SecretLister
	secretSynced cache.InformerSynced

	plantControl ControlInterface
	plantLister  gardencorelisters.PlantLister
	plantSynced  cache.InformerSynced

	plantQueue workqueue.RateLimitingInterface

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewController instantiates a new Plant controller.
func NewController(k8sGardenClient kubernetes.Interface,
	gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	config *config.ControllerManagerConfiguration,
	recorder record.EventRecorder) *Controller {
	var (
		gardenCoreInformer = gardenCoreInformerFactory.Core().V1alpha1()
		kubeInfomer        = kubeInformerFactory.Core().V1()

		plantInformer = gardenCoreInformer.Plants()
		plantLister   = plantInformer.Lister()

		secretInformer = kubeInfomer.Secrets()
		secretLister   = secretInformer.Lister()

		plantQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "plant")
	)

	controller := &Controller{
		k8sInformers: kubeInformerFactory,

		config:   config,
		recorder: recorder,

		secretLister: secretLister,
		plantLister:  plantLister,
		plantQueue:   plantQueue,
		plantControl: NewDefaultPlantControl(k8sGardenClient, recorder, config, plantLister, secretLister),

		workerCh: make(chan int),
	}

	controller.plantSynced = plantInformer.Informer().HasSynced
	plantInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.plantAdd,
		UpdateFunc: controller.plantUpdate,
		DeleteFunc: controller.plantDelete,
	})

	controller.secretSynced = secretInformer.Informer().HasSynced
	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.reconcilePlantForMatchingSecret,
		UpdateFunc: controller.plantSecretUpdate,
		DeleteFunc: controller.reconcilePlantForMatchingSecret,
	})

	return controller
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.plantSynced, c.secretSynced) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	go func() {
		for {
			select {
			case res := <-c.workerCh:
				c.numberOfRunningWorkers += res
				logger.Logger.Debugf("Current number of running Plant workers is %d", c.numberOfRunningWorkers)
			}
		}
	}()

	logger.Logger.Info("Plant controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.plantQueue, "plant", func(key string) error { return c.reconcilePlantKey(ctx, key) }, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.plantQueue.ShutDown()

	for {
		if c.plantQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running Plant worker and no items left in the queues. Terminating Plant controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d Plant worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.plantQueue.Len())
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
	metric, err := prometheus.NewConstMetric(gardenmetrics.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "plant")
	if err != nil {
		gardenmetrics.ScrapeFailures.With(prometheus.Labels{"kind": "plant-controller"}).Inc()
		return
	}
	ch <- metric
}
