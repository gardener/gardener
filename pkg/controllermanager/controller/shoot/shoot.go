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

package shoot

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
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller controls Shoots.
type Controller struct {
	config *config.ControllerManagerConfiguration

	shootHibernationReconciler reconcile.Reconciler
	shootMaintenanceReconciler reconcile.Reconciler
	shootQuotaReconciler       reconcile.Reconciler
	shootRefReconciler         reconcile.Reconciler
	shootRetryReconciler       reconcile.Reconciler
	configMapReconciler        reconcile.Reconciler
	hasSyncedFuncs             []cache.InformerSynced

	shootMaintenanceQueue  workqueue.RateLimitingInterface
	shootQuotaQueue        workqueue.RateLimitingInterface
	shootHibernationQueue  workqueue.RateLimitingInterface
	shootReferenceQueue    workqueue.RateLimitingInterface
	shootRetryQueue        workqueue.RateLimitingInterface
	configMapQueue         workqueue.RateLimitingInterface
	numberOfRunningWorkers int
	workerCh               chan int
}

// NewShootController takes a ClientMap, a GardenerInformerFactory, a KubernetesInformerFactory, a
// ControllerManagerConfig struct and an EventRecorder to create a new Shoot controller.
func NewShootController(
	ctx context.Context,
	clientMap clientmap.ClientMap,
	config *config.ControllerManagerConfiguration,
	recorder record.EventRecorder,
) (
	*Controller,
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
	configMapInformer, err := gardenClient.Cache().GetInformer(ctx, &corev1.ConfigMap{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap Informer: %w", err)
	}

	shootController := &Controller{
		config: config,

		shootHibernationReconciler: NewShootHibernationReconciler(logger.Logger, gardenClient.Client(), NewHibernationScheduleRegistry(), recorder),
		shootMaintenanceReconciler: NewShootMaintenanceReconciler(logger.Logger, gardenClient.Client(), config.Controllers.ShootMaintenance, recorder),
		shootQuotaReconciler:       NewShootQuotaReconciler(logger.Logger, gardenClient.Client(), config.Controllers.ShootQuota),
		configMapReconciler:        NewConfigMapReconciler(logger.Logger, gardenClient.Client()),
		shootRetryReconciler:       NewShootRetryReconciler(logger.Logger, gardenClient.Client(), config.Controllers.ShootRetry),
		shootRefReconciler:         NewShootReferenceReconciler(logger.Logger, gardenClient, config.Controllers.ShootReference),

		shootMaintenanceQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-maintenance"),
		shootQuotaQueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-quota"),
		shootHibernationQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-hibernation"),
		shootReferenceQueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-references"),
		shootRetryQueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-retry"),
		configMapQueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "configmaps"),

		workerCh: make(chan int),
	}

	shootInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.shootMaintenanceAdd,
		UpdateFunc: shootController.shootMaintenanceUpdate,
		DeleteFunc: shootController.shootMaintenanceDelete,
	})

	shootInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.shootQuotaAdd,
		DeleteFunc: shootController.shootQuotaDelete,
	})

	shootInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.shootHibernationAdd,
		UpdateFunc: shootController.shootHibernationUpdate,
		DeleteFunc: shootController.shootHibernationDelete,
	})

	configMapInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.configMapAdd,
		UpdateFunc: shootController.configMapUpdate,
	})

	shootInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.shootReferenceAdd,
		UpdateFunc: shootController.shootReferenceUpdate,
	})

	shootInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.shootRetryAdd,
		UpdateFunc: shootController.shootRetryUpdate,
	})

	shootController.hasSyncedFuncs = []cache.InformerSynced{
		shootInformer.HasSynced,
		configMapInformer.HasSynced,
	}

	return shootController, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, shootMaintenanceWorkers, shootQuotaWorkers, shootHibernationWorkers, shootReferenceWorkers, shootRetryWorkers int) {
	var waitGroup sync.WaitGroup
	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running Shoot workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("Shoot controller initialized.")

	for i := 0; i < shootMaintenanceWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.shootMaintenanceQueue, "Shoot Maintenance", c.shootMaintenanceReconciler, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootQuotaWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.shootQuotaQueue, "Shoot Quota", c.shootQuotaReconciler, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootHibernationWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.shootHibernationQueue, "Shoot Hibernation", c.shootHibernationReconciler, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootMaintenanceWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.configMapQueue, "ConfigMap", c.configMapReconciler, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootReferenceWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.shootReferenceQueue, "ShootReference", c.shootRefReconciler, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootRetryWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.shootRetryQueue, "Shoot Retry", c.shootRetryReconciler, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.shootMaintenanceQueue.ShutDown()
	c.shootQuotaQueue.ShutDown()
	c.shootHibernationQueue.ShutDown()
	c.configMapQueue.ShutDown()
	c.shootReferenceQueue.ShutDown()
	c.shootRetryQueue.ShutDown()

	for {
		var (
			shootMaintenanceQueueLength = c.shootMaintenanceQueue.Len()
			shootQuotaQueueLength       = c.shootQuotaQueue.Len()
			shootHibernationQueueLength = c.shootHibernationQueue.Len()
			configMapQueueLength        = c.configMapQueue.Len()
			referenceQueueLength        = c.shootReferenceQueue.Len()
			shootRetryQueueLength       = c.shootReferenceQueue.Len()
			queueLengths                = shootMaintenanceQueueLength + shootQuotaQueueLength + shootHibernationQueueLength + configMapQueueLength + referenceQueueLength + shootRetryQueueLength
		)
		if queueLengths == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running Shoot worker and no items left in the queues. Terminated Shoot controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d Shoot worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, queueLengths)
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
	metric, err := prometheus.NewConstMetric(controllermanager.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "shoot")
	if err != nil {
		controllermanager.ScrapeFailures.With(prometheus.Labels{"kind": "shoot-controller"}).Inc()
		return
	}
	ch <- metric
}
