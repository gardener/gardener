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

package backupbucket

import (
	"context"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	controllerutils "github.com/gardener/gardener/pkg/controllermanager/controller/utils"
	gardenmetrics "github.com/gardener/gardener/pkg/controllermanager/metrics"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

const (
	// finalizerName is the backupbucket controller finalizer.
	finalizerName = "core.gardener.cloud/backupbucket"
)

// Controller controls BackupBuckets.
type Controller struct {
	config     *config.ControllerManagerConfiguration
	reconciler reconcile.Reconciler
	recorder   record.EventRecorder

	backupBucketQueue  workqueue.RateLimitingInterface
	backupBucketSynced cache.InformerSynced

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewBackupBucketController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <backupBucketInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewBackupBucketController(k8sGardenClient kubernetes.Interface, k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory, config *config.ControllerManagerConfiguration, recorder record.EventRecorder) *Controller {
	var (
		gardencorev1alpha1Informer = k8sGardenCoreInformers.Core().V1alpha1()
		backupBucketInformer       = gardencorev1alpha1Informer.BackupBuckets()
	)

	backupBucketController := &Controller{
		config:            config,
		reconciler:        newReconciler(context.TODO(), k8sGardenClient.Client(), recorder),
		recorder:          recorder,
		backupBucketQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "BackupBucket"),
		workerCh:          make(chan int),
	}

	backupBucketInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    backupBucketController.backupBucketAdd,
		UpdateFunc: backupBucketController.backupBucketUpdate,
		DeleteFunc: backupBucketController.backupBucketDelete,
	})

	backupBucketController.backupBucketSynced = backupBucketInformer.Informer().HasSynced

	return backupBucketController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.backupBucketSynced) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for {
			select {
			case res := <-c.workerCh:
				c.numberOfRunningWorkers += res
				logger.Logger.Debugf("Current number of running BackupBucket workers is %d", c.numberOfRunningWorkers)
			}
		}
	}()

	logger.Logger.Info("BackupBucket controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.backupBucketQueue, "backupbucket", c.reconciler, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.backupBucketQueue.ShutDown()

	for {
		if c.backupBucketQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Info("No running BackupBucket worker and no items left in the queues. Terminated BackupBucket controller...")
			break
		}
		logger.Logger.Infof("Waiting for %d BackupBucket worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.backupBucketQueue.Len())
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
	metric, err := prometheus.NewConstMetric(gardenmetrics.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "backupbucket")
	if err != nil {
		gardenmetrics.ScrapeFailures.With(prometheus.Labels{"kind": "backupbucket-controller"}).Inc()
		return
	}
	ch <- metric
}
