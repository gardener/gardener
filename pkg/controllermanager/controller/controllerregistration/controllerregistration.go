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

package controllerregistration

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// FinalizerName is the finalizer used by this controller.
const FinalizerName = "core.gardener.cloud/controllerregistration"

// Controller controls ControllerRegistration.
type Controller struct {
	gardenClient client.Client

	controllerRegistrationReconciler     reconcile.Reconciler
	controllerRegistrationSeedReconciler reconcile.Reconciler
	seedReconciler                       reconcile.Reconciler
	hasSyncedFuncs                       []cache.InformerSynced

	controllerRegistrationQueue     workqueue.RateLimitingInterface
	controllerRegistrationSeedQueue workqueue.RateLimitingInterface
	seedQueue                       workqueue.RateLimitingInterface
	workerCh                        chan int
	numberOfRunningWorkers          int
}

// NewController instantiates a new ControllerRegistration controller.
func NewController(ctx context.Context, clientMap clientmap.ClientMap) (*Controller, error) {
	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, err
	}

	backupBucketInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.BackupBucket{})
	if err != nil {
		return nil, fmt.Errorf("failed to get BackupBucket Informer: %w", err)
	}
	backupEntryInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.BackupEntry{})
	if err != nil {
		return nil, fmt.Errorf("failed to get BackupEntry Informer: %w", err)
	}
	controllerDeploymentInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.ControllerDeployment{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ControllerDeployment Informer: %w", err)
	}
	controllerInstallationInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.ControllerInstallation{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ControllerInstallation Informer: %w", err)
	}
	controllerRegistrationInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.ControllerRegistration{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ControllerRegistration Informer: %w", err)
	}
	seedInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Seed{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Seed Informer: %w", err)
	}
	shootInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Shoot{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Shoot Informer: %w", err)
	}

	controller := &Controller{
		gardenClient: gardenClient.Client(),

		controllerRegistrationReconciler:     NewControllerRegistrationReconciler(logger.Logger, gardenClient.Client()),
		controllerRegistrationSeedReconciler: NewControllerRegistrationSeedReconciler(logger.Logger, gardenClient),
		seedReconciler:                       NewSeedReconciler(logger.Logger, gardenClient.Client()),

		controllerRegistrationQueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "controllerregistration"),
		controllerRegistrationSeedQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "controllerregistration-seed"),
		seedQueue:                       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "seed"),
		workerCh:                        make(chan int),
	}

	backupBucketInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.backupBucketAdd,
		UpdateFunc: controller.backupBucketUpdate,
		DeleteFunc: controller.backupBucketDelete,
	})

	backupEntryInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.backupEntryAdd,
		UpdateFunc: controller.backupEntryUpdate,
		DeleteFunc: controller.backupEntryDelete,
	})

	controllerRegistrationInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { controller.controllerRegistrationAdd(ctx, obj) },
		UpdateFunc: func(oldObj, newObj interface{}) { controller.controllerRegistrationUpdate(ctx, oldObj, newObj) },
		DeleteFunc: controller.controllerRegistrationDelete,
	})

	controllerDeploymentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { controller.controllerDeploymentAdd(ctx, obj) },
		UpdateFunc: func(oldObj, newObj interface{}) { controller.controllerDeploymentUpdate(ctx, oldObj, newObj) },
	})

	controllerInstallationInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.controllerInstallationAdd,
		UpdateFunc: controller.controllerInstallationUpdate,
	})

	seedInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { controller.seedAdd(obj, true) },
		UpdateFunc: controller.seedUpdate,
		DeleteFunc: controller.seedDelete,
	})

	shootInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.shootAdd,
		UpdateFunc: controller.shootUpdate,
		DeleteFunc: controller.shootDelete,
	})

	controller.hasSyncedFuncs = append(controller.hasSyncedFuncs,
		backupBucketInformer.HasSynced,
		backupEntryInformer.HasSynced,
		controllerRegistrationInformer.HasSynced,
		controllerDeploymentInformer.HasSynced,
		controllerInstallationInformer.HasSynced,
		seedInformer.HasSynced,
		shootInformer.HasSynced,
	)

	return controller, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running ControllerRegistration workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("ControllerRegistration controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.controllerRegistrationQueue, "ControllerRegistration", c.controllerRegistrationReconciler, &waitGroup, c.workerCh)
		controllerutils.CreateWorker(ctx, c.controllerRegistrationSeedQueue, "ControllerRegistration-Seed", c.controllerRegistrationSeedReconciler, &waitGroup, c.workerCh)
		controllerutils.CreateWorker(ctx, c.seedQueue, "Seed", c.seedReconciler, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.controllerRegistrationQueue.ShutDown()
	c.controllerRegistrationSeedQueue.ShutDown()
	c.seedQueue.ShutDown()

	for {
		if c.controllerRegistrationQueue.Len() == 0 && c.seedQueue.Len() == 0 && c.controllerRegistrationSeedQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running ControllerRegistration worker and no items left in the queues. Terminated ControllerRegistration controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d ControllerRegistration worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.controllerRegistrationQueue.Len()+c.seedQueue.Len()+c.controllerRegistrationSeedQueue.Len())
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
	metric, err := prometheus.NewConstMetric(controllermanager.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "controllerregistration")
	if err != nil {
		controllermanager.ScrapeFailures.With(prometheus.Labels{"kind": "controllerregistration-controller"}).Inc()
		return
	}
	ch <- metric
}

func (c *Controller) enqueueAllSeeds(ctx context.Context) {
	seedList := &metav1.PartialObjectMetadataList{}
	seedList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("SeedList"))
	if err := c.gardenClient.List(ctx, seedList); err != nil {
		return
	}

	for _, seed := range seedList.Items {
		c.controllerRegistrationSeedQueue.Add(seed.Name)
	}
}
