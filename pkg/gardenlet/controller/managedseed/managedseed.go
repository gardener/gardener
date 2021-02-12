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

package managedseed

import (
	"context"
	"fmt"
	"sync"
	"time"

	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/utils/imagevector"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller controls ManagedSeeds.
type Controller struct {
	gardenClient client.Client
	config       *config.GardenletConfiguration
	reconciler   reconcile.Reconciler

	managedSeedInformer runtimecache.Informer
	managedSeedQueue    workqueue.RateLimitingInterface

	numberOfRunningWorkers int
	workerCh               chan int

	logger *logrus.Logger
}

// NewManagedSeedController creates a new Gardener controller for ManagedSeeds.
func NewManagedSeedController(ctx context.Context, clientMap clientmap.ClientMap, config *config.GardenletConfiguration, imageVector imagevector.ImageVector, recorder record.EventRecorder, logger *logrus.Logger) (*Controller, error) {
	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, fmt.Errorf("could not get garden client: %w", err)
	}

	managedSeedInformer, err := gardenClient.Cache().GetInformer(ctx, &seedmanagementv1alpha1.ManagedSeed{})
	if err != nil {
		return nil, fmt.Errorf("could not get ManagedSeed informer: %w", err)
	}

	return &Controller{
		gardenClient:        gardenClient.Client(),
		config:              config,
		reconciler:          newReconciler(gardenClient, newActuator(gardenClient, clientMap, newValuesHelper(config, imageVector), logger), recorder, logger),
		managedSeedInformer: managedSeedInformer,
		managedSeedQueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ManagedSeed"),
		workerCh:            make(chan int),
		logger:              logger,
	}, nil
}

// Run runs the Controller until the given context is cancelled.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	c.managedSeedInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.ManagedSeedFilterFunc(ctx, c.gardenClient, confighelper.SeedNameFromSeedConfig(c.config.SeedConfig), c.config.SeedSelector),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    c.managedSeedAdd,
			UpdateFunc: c.managedSeedUpdate,
			DeleteFunc: c.managedSeedDelete,
		},
	})

	if !cache.WaitForCacheSync(ctx.Done(), c.managedSeedInformer.HasSynced) {
		c.logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			c.logger.Debugf("Current number of running ManagedSeed workers is %d", c.numberOfRunningWorkers)
		}
	}()

	c.logger.Info("ManagedSeed controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.managedSeedQueue, "ManagedSeed", c.reconciler, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.managedSeedQueue.ShutDown()

	for {
		if c.managedSeedQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			c.logger.Debug("No running ManagedSeed worker and no items left in the queues. Terminated ManagedSeed controller...")
			break
		}
		c.logger.Debugf("Waiting for %d ManagedSeed worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.managedSeedQueue.Len())
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
