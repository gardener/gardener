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

package bastion

import (
	"context"
	"fmt"
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllermanager"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// ControllerName is the name of this controller.
	ControllerName = "bastion-controller"
)

// Controller controls Bastions.
type Controller struct {
	reconciler     reconcile.Reconciler
	hasSyncedFuncs []cache.InformerSynced

	gardenClient           client.Client
	bastionQueue           workqueue.RateLimitingInterface
	workerCh               chan int
	numberOfRunningWorkers int
	maxLifetime            time.Duration
}

// NewBastionController takes a Kubernetes client <k8sGardenClient> and a <k8sGardenCoreInformers> for the Garden clusters.
// It creates and returns a new Garden controller to control Bastions.
func NewBastionController(
	ctx context.Context,
	clientMap clientmap.ClientMap,
	maxLifetime time.Duration,
) (
	*Controller,
	error,
) {
	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, err
	}

	bastionInformer, err := gardenClient.Cache().GetInformer(ctx, &operationsv1alpha1.Bastion{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Bastion Informer: %w", err)
	}

	shootInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Shoot{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Shoot Informer: %w", err)
	}

	logger := logger.Logger.WithField("controller", ControllerName)
	bastionController := &Controller{
		gardenClient: gardenClient.Client(),
		bastionQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "bastion"),
		reconciler:   NewBastionReconciler(logger, gardenClient.Client(), maxLifetime),
		workerCh:     make(chan int),
		maxLifetime:  maxLifetime,
	}

	bastionInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    bastionController.bastionAdd,
		UpdateFunc: bastionController.bastionUpdate,
		DeleteFunc: bastionController.bastionDelete,
	})

	shootInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { bastionController.shootAdd(ctx, obj) },
		UpdateFunc: func(old, new interface{}) { bastionController.shootUpdate(ctx, old, new) },
		DeleteFunc: func(obj interface{}) { bastionController.shootDelete(ctx, obj) },
	})

	bastionController.hasSyncedFuncs = append(bastionController.hasSyncedFuncs, bastionInformer.HasSynced, shootInformer.HasSynced)

	return bastionController, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	// Check if informers cache has been populated
	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		logger.Logger.Error("Time out waiting for caches to sync")
		return
	}

	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running Bastion workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("Bastion controller initialized.")

	// Start the workers
	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.bastionQueue, "Bastion", c.reconciler, &waitGroup, c.workerCh)
	}

	<-ctx.Done()
	c.bastionQueue.ShutDown()

	for {
		if c.bastionQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running Bastion worker and no items left in the queues. Terminating Bastion controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d Bastion worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.bastionQueue.Len())
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
	metric, err := prometheus.NewConstMetric(controllermanager.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "bastion")
	if err != nil {
		controllermanager.ScrapeFailures.With(prometheus.Labels{"kind": ControllerName}).Inc()
		return
	}
	ch <- metric
}
