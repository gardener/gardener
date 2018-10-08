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
	"sync"
	"time"

	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllerutils "github.com/gardener/gardener/pkg/controller/utils"
	"github.com/gardener/gardener/pkg/logger"
	gardenmetrics "github.com/gardener/gardener/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// Controller controls Quotas.
type Controller struct {
	k8sGardenClient    kubernetes.Client
	k8sGardenInformers gardeninformers.SharedInformerFactory

	control  ControlInterface
	recorder record.EventRecorder

	quotaLister gardenlisters.QuotaLister
	quotaQueue  workqueue.RateLimitingInterface
	quotaSynced cache.InformerSynced

	secretBindingLister gardenlisters.SecretBindingLister

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewQuotaController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <quotaInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewQuotaController(k8sGardenClient kubernetes.Client, gardenInformerFactory gardeninformers.SharedInformerFactory, recorder record.EventRecorder) *Controller {
	var (
		gardenv1beta1Informer = gardenInformerFactory.Garden().V1beta1()

		quotaInformer       = gardenv1beta1Informer.Quotas()
		quotaLister         = quotaInformer.Lister()
		secretBindingLister = gardenv1beta1Informer.SecretBindings().Lister()
	)

	quotaController := &Controller{
		k8sGardenClient:     k8sGardenClient,
		k8sGardenInformers:  gardenInformerFactory,
		control:             NewDefaultControl(k8sGardenClient, gardenInformerFactory, recorder, secretBindingLister),
		recorder:            recorder,
		quotaLister:         quotaLister,
		quotaQueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Quota"),
		secretBindingLister: secretBindingLister,
		workerCh:            make(chan int),
	}

	quotaInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    quotaController.quotaAdd,
		UpdateFunc: quotaController.quotaUpdate,
		DeleteFunc: quotaController.quotaDelete,
	})
	quotaController.quotaSynced = quotaInformer.Informer().HasSynced

	return quotaController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.quotaSynced) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for {
			select {
			case res := <-c.workerCh:
				c.numberOfRunningWorkers += res
				logger.Logger.Debugf("Current number of running Quota workers is %d", c.numberOfRunningWorkers)
			}
		}
	}()

	logger.Logger.Info("Quota controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.quotaQueue, "Quota", c.reconcileQuotaKey, &waitGroup, c.workerCh)
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
	metric, err := prometheus.NewConstMetric(gardenmetrics.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "quota")
	if err != nil {
		gardenmetrics.ScrapeFailures.With(prometheus.Labels{"kind": "quota-controller"}).Inc()
		return
	}
	ch <- metric
}
