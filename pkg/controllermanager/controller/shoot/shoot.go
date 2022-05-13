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
	gardencorev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"
	kutils "github.com/gardener/gardener/pkg/utils/kubernetes"

	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	shootConditionsReconciler  reconcile.Reconciler
	shootStatusLabelReconciler reconcile.Reconciler
	hasSyncedFuncs             []cache.InformerSynced

	shootMaintenanceQueue  workqueue.RateLimitingInterface
	shootQuotaQueue        workqueue.RateLimitingInterface
	shootHibernationQueue  workqueue.RateLimitingInterface
	shootReferenceQueue    workqueue.RateLimitingInterface
	shootRetryQueue        workqueue.RateLimitingInterface
	shootConditionsQueue   workqueue.RateLimitingInterface
	shootStatusLabelQueue  workqueue.RateLimitingInterface
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
	seedInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Seed{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Seed Informer: %w", err)
	}

	shootController := &Controller{
		config: config,

		shootHibernationReconciler: NewShootHibernationReconciler(gardenClient.Client(), config.Controllers.ShootHibernation, recorder, clock.RealClock{}),
		shootMaintenanceReconciler: NewShootMaintenanceReconciler(logger.Logger, gardenClient.Client(), config.Controllers.ShootMaintenance, recorder),
		shootQuotaReconciler:       NewShootQuotaReconciler(logger.Logger, gardenClient.Client(), config.Controllers.ShootQuota),
		shootRetryReconciler:       NewShootRetryReconciler(logger.Logger, gardenClient.Client(), config.Controllers.ShootRetry),
		shootConditionsReconciler:  NewShootConditionsReconciler(logger.Logger, gardenClient.Client()),
		shootStatusLabelReconciler: NewShootStatusLabelReconciler(logger.Logger, gardenClient.Client()),
		shootRefReconciler:         NewShootReferenceReconciler(logger.Logger, gardenClient),

		shootMaintenanceQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-maintenance"),
		shootQuotaQueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-quota"),
		shootHibernationQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-hibernation"),
		shootReferenceQueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-references"),
		shootRetryQueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-retry"),
		shootConditionsQueue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-conditions"),
		shootStatusLabelQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-status-label"),

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
	})

	shootInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.shootReferenceAdd,
		UpdateFunc: shootController.shootReferenceUpdate,
	})

	shootInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.shootRetryAdd,
		UpdateFunc: shootController.shootRetryUpdate,
	})

	shootInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: shootController.shootConditionsAdd,
	})

	// Add event handler for seeds that are registered via managed seeds referencing shoots
	seedInformer.AddEventHandler(&kutils.ControlledResourceEventHandler{
		ControllerTypes: []kutils.ControllerType{
			{
				Type:      &seedmanagementv1alpha1.ManagedSeed{},
				Namespace: pointer.String(gardencorev1beta1constants.GardenNamespace),
				NameFunc:  func(obj client.Object) string { return obj.GetName() },
			},
			{
				Type:      &gardencorev1beta1.Shoot{},
				Namespace: pointer.String(gardencorev1beta1constants.GardenNamespace),
				NameFunc: func(obj client.Object) string {
					ms, ok := obj.(*seedmanagementv1alpha1.ManagedSeed)
					if !ok || ms.Spec.Shoot == nil {
						return ""
					}
					return ms.Spec.Shoot.Name
				},
			},
		},
		Ctx:                        ctx,
		Reader:                     gardenClient.Cache(),
		ControllerPredicateFactory: kutils.ControllerPredicateFactoryFunc(FilterSeedForShootConditions),
		Enqueuer:                   kutils.EnqueuerFunc(func(obj client.Object) { shootController.shootConditionsAdd(obj) }),
		Scheme:                     kubernetes.GardenScheme,
		Logger:                     logger.Logger,
	})

	shootInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.shootStatusLabelAdd,
		UpdateFunc: shootController.shootStatusLabelUpdate,
	})

	shootController.hasSyncedFuncs = []cache.InformerSynced{
		shootInformer.HasSynced,
		seedInformer.HasSynced,
	}

	return shootController, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(
	ctx context.Context,
	shootMaintenanceWorkers, shootQuotaWorkers, shootHibernationWorkers, shootReferenceWorkers, shootRetryWorkers, shootConditionsWorkers, shootStatusLabelWorkers int,
) {
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
	for i := 0; i < shootReferenceWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.shootReferenceQueue, "ShootReference", c.shootRefReconciler, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootRetryWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.shootRetryQueue, "Shoot Retry", c.shootRetryReconciler, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootConditionsWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.shootConditionsQueue, "Shoot Conditions", c.shootConditionsReconciler, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootStatusLabelWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.shootStatusLabelQueue, "Shoot Status Label", c.shootStatusLabelReconciler, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.shootMaintenanceQueue.ShutDown()
	c.shootQuotaQueue.ShutDown()
	c.shootHibernationQueue.ShutDown()
	c.shootReferenceQueue.ShutDown()
	c.shootRetryQueue.ShutDown()
	c.shootConditionsQueue.ShutDown()
	c.shootStatusLabelQueue.ShutDown()

	for {
		var (
			shootMaintenanceQueueLength = c.shootMaintenanceQueue.Len()
			shootQuotaQueueLength       = c.shootQuotaQueue.Len()
			shootHibernationQueueLength = c.shootHibernationQueue.Len()
			referenceQueueLength        = c.shootReferenceQueue.Len()
			shootRetryQueueLength       = c.shootRetryQueue.Len()
			shootConditionsQueueLength  = c.shootConditionsQueue.Len()
			shootStatusLabelQueueLength = c.shootStatusLabelQueue.Len()
			queueLengths                = shootMaintenanceQueueLength + shootQuotaQueueLength + shootHibernationQueueLength + referenceQueueLength + shootRetryQueueLength + shootConditionsQueueLength + shootStatusLabelQueueLength
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
