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
	"fmt"
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllermanager"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// FinalizerName is the name of the Plant finalizer.
const FinalizerName = "core.gardener.cloud/plant"

// Controller controls Plant.
type Controller struct {
	gardenClient client.Client

	reconciler     reconcile.Reconciler
	hasSyncedFuncs []cache.InformerSynced

	plantQueue             workqueue.RateLimitingInterface
	workerCh               chan int
	numberOfRunningWorkers int
}

// NewController instantiates a new Plant controller.
func NewController(
	ctx context.Context,
	clientMap clientmap.ClientMap,
	config *config.ControllerManagerConfiguration,
) (
	*Controller,
	error,
) {
	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, err
	}

	plantInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Plant{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Plant Informer: %w", err)
	}
	secretInformer, err := gardenClient.Cache().GetInformer(ctx, &corev1.Secret{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Secret Informer: %w", err)
	}

	controller := &Controller{
		gardenClient: gardenClient.Client(),
		reconciler:   NewPlantReconciler(logger.Logger, clientMap, gardenClient.Client(), config.Controllers.Plant),
		plantQueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "plant"),
		workerCh:     make(chan int),
	}

	plantInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.plantAdd,
		UpdateFunc: controller.plantUpdate,
		DeleteFunc: controller.plantDelete,
	})

	secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { controller.reconcilePlantForMatchingSecret(ctx, obj) },
		UpdateFunc: func(oldObj, newObj interface{}) { controller.plantSecretUpdate(ctx, oldObj, newObj) },
		DeleteFunc: func(obj interface{}) { controller.reconcilePlantForMatchingSecret(ctx, obj) },
	})

	controller.hasSyncedFuncs = append(controller.hasSyncedFuncs, plantInformer.HasSynced, secretInformer.HasSynced)

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
			logger.Logger.Debugf("Current number of running Plant workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("Plant controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.plantQueue, "Plant", c.reconciler, &waitGroup, c.workerCh)
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
	metric, err := prometheus.NewConstMetric(controllermanager.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "plant")
	if err != nil {
		controllermanager.ScrapeFailures.With(prometheus.Labels{"kind": "plant-controller"}).Inc()
		return
	}
	ch <- metric
}
