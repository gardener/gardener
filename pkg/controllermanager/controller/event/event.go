// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package event

import (
	"context"
	"sync"
	"time"

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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller controls Events.
type Controller struct {
	clientMap              clientmap.ClientMap
	cfg                    *config.EventControllerConfiguration
	eventQueue             workqueue.RateLimitingInterface
	numberOfRunningWorkers int
	workerCh               chan int
}

// NewController instantiates a new event controller
func NewController(clientMap clientmap.ClientMap, cfg *config.EventControllerConfiguration) *Controller {
	controller := &Controller{
		clientMap:  clientMap,
		cfg:        cfg,
		eventQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "event"),
		workerCh:   make(chan int),
	}

	return controller
}

// Run runs the Controller
func (c *Controller) Run(ctx context.Context) {
	var waitGroup sync.WaitGroup

	if c.cfg == nil {
		return
	}

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		logger.Logger.Errorf("failed to get garden client: %v", err)
		return
	}

	eventInformer, err := gardenClient.Cache().GetInformer(ctx, &corev1.Event{})
	if err != nil {
		logger.Logger.Errorf("failed to get event informer: %v", err)
		return
	}

	eventInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueEvent,
	})

	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running Event workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("Event controller initialized.")

	for i := 0; i < c.cfg.ConcurrentSyncs; i++ {
		controllerutils.CreateWorker(ctx, c.eventQueue, "Event", reconcile.Func(c.reconcileEvent), &waitGroup, c.workerCh)
	}

	<-ctx.Done()
	c.eventQueue.ShutDown()

	for {
		if c.eventQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running Event worker and no items left in the queues. Terminating Event controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d Event worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.eventQueue.Len())
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
	metric, err := prometheus.NewConstMetric(controllermanager.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "event")
	if err != nil {
		controllermanager.ScrapeFailures.With(prometheus.Labels{"kind": "event-controller"}).Inc()
		return
	}
	ch <- metric
}
