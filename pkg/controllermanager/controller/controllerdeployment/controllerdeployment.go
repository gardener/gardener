// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controllerdeployment

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet"
)

// FinalizerName is the finalizer used by this controller.
const FinalizerName = "core.gardener.cloud/controllerdeployment"

// Controller controls ManagedSeedSets.
type Controller struct {
	controllerDeploymentReconciler reconcile.Reconciler
	hasSyncedFuncs                 []cache.InformerSynced

	controllerDeploymentQueue workqueue.RateLimitingInterface

	numberOfRunningWorkers int
	workerCh               chan int

	logger *logrus.Logger
}

// New creates a new Gardener controller for ControllerDeployments.
func New(
	ctx context.Context,
	clientMap clientmap.ClientMap,
	logger *logrus.Logger,
) (*Controller, error) {
	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, fmt.Errorf("could not get garden client: %w", err)
	}

	controllerDeploymentInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.ControllerDeployment{})
	if err != nil {
		return nil, fmt.Errorf("could not get ControllerDeployment informer: %w", err)
	}

	controller := &Controller{
		controllerDeploymentReconciler: NewReconciler(logger, gardenClient.Client()),
		controllerDeploymentQueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ManagedSeedSet"),
		workerCh:                       make(chan int),
		logger:                         logger,
	}

	controllerDeploymentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.controllerDeploymentAdd,
		UpdateFunc: controller.controllerDeploymentUpdate,
	})

	controller.hasSyncedFuncs = []cache.InformerSynced{
		controllerDeploymentInformer.HasSynced,
	}

	return controller, nil
}

// Run runs the Controller until the given context is cancelled.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		c.logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			c.logger.Debugf("Current number of running ControllerDeployment workers is %d", c.numberOfRunningWorkers)
		}
	}()

	c.logger.Info("ControllerDeployment controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.controllerDeploymentQueue, "ControllerDeployment", c.controllerDeploymentReconciler, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.controllerDeploymentQueue.ShutDown()

	for {
		if c.controllerDeploymentQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			c.logger.Debug("No running ControllerDeployment worker and no items left in the queues. Terminated ControllerDeployment controller...")
			break
		}
		c.logger.Debugf("Waiting for %d ControllerDeployment worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.controllerDeploymentQueue.Len())
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
	metric, err := prometheus.NewConstMetric(gardenlet.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "controllerDeployment")
	if err != nil {
		gardenlet.ScrapeFailures.With(prometheus.Labels{"kind": "controllerDeployment-controller"}).Inc()
		return
	}
	ch <- metric
}
