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

package seed

import (
	"context"
	"fmt"
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller controls Seeds.
type Controller struct {
	gardenClient client.Client
	config       *config.ControllerManagerConfiguration

	seedReconciler       reconcile.Reconciler
	lifeCycleReconciler  reconcile.Reconciler
	seedBackupReconciler reconcile.Reconciler
	hasSyncedFuncs       []cache.InformerSynced

	seedBackupBucketQueue workqueue.RateLimitingInterface
	seedQueue             workqueue.RateLimitingInterface
	seedLifecycleQueue    workqueue.RateLimitingInterface

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewSeedController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <gardenInformerFactory>, and a <recorder> for
// event recording. It creates a new Seed controller.
func NewSeedController(
	ctx context.Context,
	clientMap clientmap.ClientMap,
	config *config.ControllerManagerConfiguration,
) (
	*Controller,
	error,
) {
	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, err
	}

	backupBucketInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.BackupBucket{})
	if err != nil {
		return nil, fmt.Errorf("failed to get BackupBucket Informer: %w", err)
	}
	secretInformer, err := gardenClient.Cache().GetInformer(ctx, &corev1.Secret{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Secret Informer: %w", err)
	}
	seedInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Seed{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Seed Informer: %w", err)
	}

	seedController := &Controller{
		gardenClient: gardenClient.Client(),
		config:       config,

		seedReconciler:       NewDefaultControl(logger.Logger, gardenClient.Client()),
		lifeCycleReconciler:  NewLifecycleDefaultControl(logger.Logger, gardenClient, config),
		seedBackupReconciler: NewDefaultBackupBucketControl(logger.Logger, gardenClient),

		seedBackupBucketQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Backup Bucket"),
		seedLifecycleQueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Seed Lifecycle"),
		seedQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Seed"),
		workerCh:              make(chan int),
	}

	backupBucketInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    seedController.backupBucketAdd,
		UpdateFunc: seedController.backupBucketUpdate,
	})

	seedInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: seedController.seedLifecycleAdd,
	})

	seedInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: seedController.seedAdd,
	})

	secretInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: filterGardenSecret,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { seedController.gardenSecretAdd(ctx, obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { seedController.gardenSecretUpdate(ctx, oldObj, newObj) },
			DeleteFunc: func(obj interface{}) { seedController.gardenSecretDelete(ctx, obj) },
		},
	})

	seedController.hasSyncedFuncs = []cache.InformerSynced{
		backupBucketInformer.HasSynced,
		seedInformer.HasSynced,
		secretInformer.HasSynced,
	}

	return seedController, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running Seed workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("Seed controller initialized.")

	var waitGroup sync.WaitGroup
	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.seedQueue, "Seed", c.seedReconciler, &waitGroup, c.workerCh)
		controllerutils.CreateWorker(ctx, c.seedLifecycleQueue, "Seed Lifecycle", c.lifeCycleReconciler, &waitGroup, c.workerCh)
		controllerutils.CreateWorker(ctx, c.seedBackupBucketQueue, "Seed Backup Bucket", c.seedBackupReconciler, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.seedQueue.ShutDown()
	c.seedBackupBucketQueue.ShutDown()
	c.seedLifecycleQueue.ShutDown()

	for {
		if c.seedQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running Seed worker and no items left in the queues. Terminated Seed controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d Seed worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.seedQueue.Len())
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
	metric, err := prometheus.NewConstMetric(controllermanager.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "seed")
	if err != nil {
		controllermanager.ScrapeFailures.With(prometheus.Labels{"kind": "seed-controller"}).Inc()
		return
	}
	ch <- metric
}

func reconcileAfter(d time.Duration) (reconcile.Result, error) {
	return reconcile.Result{RequeueAfter: d}, nil
}

func reconcileResult(err error) (reconcile.Result, error) {
	return reconcile.Result{}, err
}
