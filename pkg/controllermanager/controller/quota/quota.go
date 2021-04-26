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

package quota

import (
	"context"
	"fmt"
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllermanager"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller controls Quotas.
type Controller struct {
	reconciler     reconcile.Reconciler
	hasSyncedFuncs []cache.InformerSynced

	quotaQueue             workqueue.RateLimitingInterface
	workerCh               chan int
	numberOfRunningWorkers int
}

// NewQuotaController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <quotaInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewQuotaController(
	ctx context.Context,
	clientMap clientmap.ClientMap,
	recorder record.EventRecorder,
) (
	*Controller,
	error,
) {
	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, err
	}

	quotaInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Quota{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Quota Informer: %w", err)
	}

	quotaController := &Controller{
		reconciler: NewQuotaReconciler(logger.Logger, gardenClient.Client(), recorder),
		quotaQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Quota"),
		workerCh:   make(chan int),
	}

	quotaInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    quotaController.quotaAdd,
		UpdateFunc: quotaController.quotaUpdate,
		DeleteFunc: quotaController.quotaDelete,
	})

	quotaController.hasSyncedFuncs = append(quotaController.hasSyncedFuncs, quotaInformer.HasSynced)

	return quotaController, nil
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
			logger.Logger.Debugf("Current number of running Quota workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("Quota controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.quotaQueue, "Quota", c.reconciler, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.quotaQueue.ShutDown()

	for {
		if c.quotaQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running Quota worker and no items left in the queues. Terminated Quota controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d Quota worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.quotaQueue.Len())
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
	metric, err := prometheus.NewConstMetric(controllermanager.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "quota")
	if err != nil {
		controllermanager.ScrapeFailures.With(prometheus.Labels{"kind": "quota-controller"}).Inc()
		return
	}
	ch <- metric
}
