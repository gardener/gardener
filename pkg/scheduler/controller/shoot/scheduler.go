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

package shoot

import (
	"context"
	"fmt"
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/scheduler"
	"github.com/gardener/gardener/pkg/scheduler/apis/config"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// SchedulerController controls Seeds.
type SchedulerController struct {
	config         *config.SchedulerConfiguration
	reconciler     reconcile.Reconciler
	hasSyncedFuncs []cache.InformerSynced

	shootQueue             workqueue.RateLimitingInterface
	workerCh               chan int
	numberOfRunningWorkers int
}

// NewGardenerScheduler creates a new scheduler controller.
func NewGardenerScheduler(
	ctx context.Context,
	clientMap clientmap.ClientMap,
	config *config.SchedulerConfiguration,
	recorder record.EventRecorder,
) (
	*SchedulerController,
	error,
) {
	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, err
	}

	shootInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Shoot{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Shoot Informer: %w", err)
	}

	schedulerController := &SchedulerController{
		reconciler: NewReconciler(logger.Logger, config, gardenClient, recorder),
		config:     config,
		shootQueue: workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(config.Schedulers.Shoot.RetrySyncPeriod.Duration, 12*time.Hour), "gardener-shoot-scheduler"),
		workerCh:   make(chan int),
	}

	shootInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    schedulerController.shootAdd,
		UpdateFunc: schedulerController.shootUpdate,
	})

	schedulerController.hasSyncedFuncs = append(schedulerController.hasSyncedFuncs, shootInformer.HasSynced)

	return schedulerController, nil
}

// Run runs the SchedulerController until the given stop channel can be read from.
func (c *SchedulerController) Run(ctx context.Context) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running Scheduler workers is %d", c.numberOfRunningWorkers)
		}
	}()

	for i := 0; i < c.config.Schedulers.Shoot.ConcurrentSyncs; i++ {
		controllerutils.CreateWorker(ctx, c.shootQueue, "Shoot", c.reconciler, &waitGroup, c.workerCh)
	}

	logger.Logger.Infof("Shoot Scheduler controller initialized with %d workers  (with Strategy: %s)", c.config.Schedulers.Shoot.ConcurrentSyncs, c.config.Schedulers.Shoot.Strategy)

	<-ctx.Done()
	c.shootQueue.ShutDown()

	for {
		if c.shootQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running Scheduler worker and no items left in the queues. Terminated Scheduler controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d Scheduler worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.shootQueue.Len())
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}

// RunningWorkers returns the number of running workers.
func (c *SchedulerController) RunningWorkers() int {
	return c.numberOfRunningWorkers
}

// CollectMetrics implements gardenmetrics.ControllerMetricsCollector interface
func (c *SchedulerController) CollectMetrics(ch chan<- prometheus.Metric) {
	metric, err := prometheus.NewConstMetric(scheduler.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "seed")
	if err != nil {
		scheduler.ScrapeFailures.With(prometheus.Labels{"kind": "gardener-shoot-scheduler"}).Inc()
		return
	}
	ch <- metric
}
