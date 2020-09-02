// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package backupentry

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
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller controls BackupEntries.
type Controller struct {
	gardenClient client.Client
	config       *config.GardenletConfiguration
	reconciler   reconcile.Reconciler
	recorder     record.EventRecorder

	backupEntryInformer runtimecache.Informer
	backupEntryQueue    workqueue.RateLimitingInterface

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewBackupEntryController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <backupEntryInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewBackupEntryController(ctx context.Context, clientMap clientmap.ClientMap, config *config.GardenletConfiguration, recorder record.EventRecorder) (*Controller, error) {
	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, fmt.Errorf("failed to get garden client: %w", err)
	}

	backupEntryInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.BackupEntry{})
	if err != nil {
		return nil, fmt.Errorf("failed to get BackupEntry Informer: %w", err)
	}

	return &Controller{
		gardenClient:        gardenClient.Client(),
		config:              config,
		reconciler:          newReconciler(ctx, clientMap, recorder, config),
		recorder:            recorder,
		backupEntryInformer: backupEntryInformer,
		backupEntryQueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "BackupEntry"),
		workerCh:            make(chan int),
	}, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	c.backupEntryInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.BackupEntryFilterFunc(ctx, c.gardenClient, confighelper.SeedNameFromSeedConfig(c.config.SeedConfig), c.config.SeedSelector),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    c.backupEntryAdd,
			UpdateFunc: c.backupEntryUpdate,
			DeleteFunc: c.backupEntryDelete,
		},
	})

	if !cache.WaitForCacheSync(ctx.Done(), c.backupEntryInformer.HasSynced) {
		logger.Logger.Fatal("Timed out waiting for BackupEntry Informer to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running BackupEntry workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("BackupEntry controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.backupEntryQueue, "backupentry", c.reconciler, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.backupEntryQueue.ShutDown()

	for {
		if c.backupEntryQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Info("No running BackupEntry worker and no items left in the queues. Terminated BackupEntry controller...")
			break
		}
		logger.Logger.Infof("Waiting for %d BackupEntry worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.backupEntryQueue.Len())
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
	metric, err := prometheus.NewConstMetric(gardenlet.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "backupentry")
	if err != nil {
		gardenlet.ScrapeFailures.With(prometheus.Labels{"kind": "backupentry-controller"}).Inc()
		return
	}
	ch <- metric
}
