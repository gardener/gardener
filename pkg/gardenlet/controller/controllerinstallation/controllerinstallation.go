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

package controllerinstallation

import (
	"context"
	"fmt"
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// FinalizerName is the name of the ControllerInstallation finalizer.
const FinalizerName = "core.gardener.cloud/controllerinstallation"

// Controller controls ControllerInstallation.
type Controller struct {
	reconciler     reconcile.Reconciler
	careReconciler reconcile.Reconciler

	controllerInstallationQueue     workqueue.RateLimitingInterface
	controllerInstallationCareQueue workqueue.RateLimitingInterface

	hasSyncedFuncs         []cache.InformerSynced
	workerCh               chan int
	numberOfRunningWorkers int
}

// NewController instantiates a new ControllerInstallation controller.
func NewController(ctx context.Context, clientMap clientmap.ClientMap, config *config.GardenletConfiguration, recorder record.EventRecorder, gardenNamespace *corev1.Namespace, gardenClusterIdentity string) (*Controller, error) {
	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, err
	}

	controllerInstallationInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.ControllerInstallation{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ControllerInstallation Informer: %w", err)
	}

	controller := &Controller{
		reconciler:     newReconciler(clientMap, recorder, logger.Logger, gardenNamespace, gardenClusterIdentity),
		careReconciler: newCareReconciler(clientMap, logger.Logger, config.Controllers.ControllerInstallationCare),

		controllerInstallationQueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "controllerinstallation"),
		controllerInstallationCareQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "controllerinstallation-care"),

		workerCh: make(chan int),
	}

	controllerInstallationInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.ControllerInstallationFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig)),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.controllerInstallationAdd,
			UpdateFunc: controller.controllerInstallationUpdate,
			DeleteFunc: controller.controllerInstallationDelete,
		},
	})

	// TODO: add a watch for ManagedResources and run the care reconciler on changed to the MR conditions
	controllerInstallationInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.ControllerInstallationFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig)),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: controller.controllerInstallationCareAdd,
		},
	})

	controller.hasSyncedFuncs = []cache.InformerSynced{
		controllerInstallationInformer.HasSynced,
	}

	return controller, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers, careWorkers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running ControllerInstallation workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("ControllerInstallation controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.controllerInstallationQueue, "ControllerInstallation", c.reconciler, &waitGroup, c.workerCh)
	}

	for i := 0; i < careWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.controllerInstallationCareQueue, "ControllerInstallation Care", c.careReconciler, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.controllerInstallationQueue.ShutDown()
	c.controllerInstallationCareQueue.ShutDown()

	for {
		if c.controllerInstallationQueue.Len() == 0 && c.controllerInstallationCareQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running ControllerInstallation worker and no items left in the queues. Terminated ControllerInstallation controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d ControllerInstallation worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.controllerInstallationQueue.Len()+c.controllerInstallationCareQueue.Len())
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
	metric, err := prometheus.NewConstMetric(gardenlet.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "controllerinstallation")
	if err != nil {
		gardenlet.ScrapeFailures.With(prometheus.Labels{"kind": "controllerinstallation-controller"}).Inc()
		return
	}
	ch <- metric
}
