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
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller controls Shoots.
type Controller struct {
	clientMap              clientmap.ClientMap
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory

	config                        *config.GardenletConfiguration
	gardenClusterIdentity         string
	identity                      *gardencorev1beta1.Gardener
	careReconciler                reconcile.Reconciler
	seedRegistrationReconciler    reconcile.Reconciler
	recorder                      record.EventRecorder
	imageVector                   imagevector.ImageVector
	shootReconciliationDueTracker *reconciliationDueTracker

	shootLister gardencorelisters.ShootLister

	shootCareQueue        workqueue.RateLimitingInterface
	shootQueue            workqueue.RateLimitingInterface
	shootSeedQueue        workqueue.RateLimitingInterface
	seedRegistrationQueue workqueue.RateLimitingInterface

	hasSyncedFuncs []cache.InformerSynced

	numberOfRunningWorkers int
	workerCh               chan int
}

// NewShootController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <shootInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewShootController(clientMap clientmap.ClientMap, k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory, config *config.GardenletConfiguration, identity *gardencorev1beta1.Gardener,
	gardenClusterIdentity string, imageVector imagevector.ImageVector, recorder record.EventRecorder) *Controller {
	var (
		gardenCoreV1beta1Informer = k8sGardenCoreInformers.Core().V1beta1()

		shootInformer = gardenCoreV1beta1Informer.Shoots()
		shootLister   = shootInformer.Lister()
	)

	shootController := &Controller{
		clientMap:              clientMap,
		k8sGardenCoreInformers: k8sGardenCoreInformers,

		config:                        config,
		identity:                      identity,
		gardenClusterIdentity:         gardenClusterIdentity,
		careReconciler:                NewCareReconciler(clientMap, imageVector, identity, gardenClusterIdentity, config),
		seedRegistrationReconciler:    NewSeedRegistrationReconciler(clientMap, recorder, logger.Logger),
		recorder:                      recorder,
		imageVector:                   imageVector,
		shootReconciliationDueTracker: newReconciliationDueTracker(),

		shootLister: shootLister,

		shootCareQueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-care"),
		shootQueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot"),
		shootSeedQueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-seeds"),
		seedRegistrationQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shooted-seed-registration"),

		workerCh: make(chan int),
	}

	shootInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.ShootFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig)),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				shootController.shootAdd(obj, false)
			},
			UpdateFunc: shootController.shootUpdate,
			DeleteFunc: shootController.shootDelete,
		},
	})

	shootInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.ShootFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig)),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    shootController.shootCareAdd,
			UpdateFunc: shootController.shootCareUpdate,
		},
	})

	shootInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.ShootFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig)),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    shootController.seedRegistrationAdd,
			UpdateFunc: shootController.seedRegistrationUpdate,
		},
	})

	shootController.hasSyncedFuncs = []cache.InformerSynced{
		shootInformer.Informer().HasSynced,
	}

	return shootController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, shootWorkers, shootCareWorkers int) {
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

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		panic(fmt.Errorf("failed to get garden client: %v", err))
	}

	// Update Shoots before starting the workers.
	shootFilterFunc := controllerutils.ShootFilterFunc(confighelper.SeedNameFromSeedConfig(c.config.SeedConfig))
	shoots, err := c.shootLister.List(labels.Everything())
	if err != nil {
		logger.Logger.Errorf("Failed to fetch shoots resources: %v", err.Error())
		return
	}
	for _, shoot := range shoots {
		if !shootFilterFunc(shoot) {
			continue
		}

		// Check if the status indicates that an operation is processing and mark it as "aborted".
		if shoot.Status.LastOperation != nil && shoot.Status.LastOperation.State == gardencorev1beta1.LastOperationStateProcessing {
			newShoot := shoot.DeepCopy()
			newShoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateAborted
			if _, err := gardenClient.GardenCore().CoreV1beta1().Shoots(newShoot.Namespace).UpdateStatus(ctx, newShoot, kubernetes.DefaultUpdateOptions()); err != nil {
				panic(fmt.Sprintf("Failed to update shoot status [%v]: %v ", newShoot.Name, err.Error()))
			}
		}
	}

	logger.Logger.Info("Shoot controller initialized.")

	for i := 0; i < shootWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.shootQueue, "Shoot", reconcile.Func(c.reconcileShootRequest), &waitGroup, c.workerCh)
	}
	for i := 0; i < shootCareWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.shootCareQueue, "Shoot Care", c.careReconciler, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootWorkers/2+1; i++ {
		controllerutils.CreateWorker(ctx, c.shootSeedQueue, "Shooted Seeds Reconciliation", reconcile.Func(c.reconcileShootRequest), &waitGroup, c.workerCh)
		controllerutils.CreateWorker(ctx, c.seedRegistrationQueue, "Shooted Seeds Registration", c.seedRegistrationReconciler, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.seedRegistrationQueue.ShutDown()
	c.shootCareQueue.ShutDown()
	c.shootQueue.ShutDown()
	c.shootSeedQueue.ShutDown()

	for {
		var (
			seedRegistrationQueueLength = c.seedRegistrationQueue.Len()
			shootQueueLength            = c.shootQueue.Len()
			shootCareQueueLength        = c.shootCareQueue.Len()
			shootSeedQueueLength        = c.shootSeedQueue.Len()
			queueLengths                = shootQueueLength + shootCareQueueLength + shootSeedQueueLength + seedRegistrationQueueLength
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
	metric, err := prometheus.NewConstMetric(gardenlet.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "shoot")
	if err != nil {
		gardenlet.ScrapeFailures.With(prometheus.Labels{"kind": "shoot-controller"}).Inc()
		return
	}
	ch <- metric
}

func (c *Controller) getShootQueue(obj interface{}) workqueue.RateLimitingInterface {
	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()
	if shoot, ok := obj.(*gardencorev1beta1.Shoot); ok && c.shootIsSeed(ctx, shoot) {
		return c.shootSeedQueue
	}
	return c.shootQueue
}

func (c *Controller) newProgressReporter(reporterFn flow.ProgressReporterFn) flow.ProgressReporter {
	if c.config.Controllers.Shoot != nil && c.config.Controllers.Shoot.ProgressReportPeriod != nil {
		return flow.NewDelayingProgressReporter(reporterFn, c.config.Controllers.Shoot.ProgressReportPeriod.Duration)
	}
	return flow.NewImmediateProgressReporter(reporterFn)
}

func (c *Controller) shootIsSeed(ctx context.Context, shoot *gardencorev1beta1.Shoot) bool {
	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return false
	}
	managedSeed, err := kutil.GetManagedSeed(ctx, gardenClient.GardenSeedManagement(), shoot.Namespace, shoot.Name)
	if err != nil {
		return false
	}
	return managedSeed != nil
}
