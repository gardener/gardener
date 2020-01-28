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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Controller controls Shoots.
type Controller struct {
	k8sGardenClient        kubernetes.Interface
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory

	config                      *config.ControllerManagerConfiguration
	recorder                    record.EventRecorder
	maintenanceControl          MaintenanceControlInterface
	quotaControl                QuotaControlInterface
	hibernationScheduleRegistry HibernationScheduleRegistry

	shootLister     gardencorelisters.ShootLister
	configMapLister kubecorev1listers.ConfigMapLister

	shootMaintenanceQueue workqueue.RateLimitingInterface
	shootQuotaQueue       workqueue.RateLimitingInterface
	shootHibernationQueue workqueue.RateLimitingInterface
	configMapQueue        workqueue.RateLimitingInterface

	shootSynced     cache.InformerSynced
	quotaSynced     cache.InformerSynced
	configMapSynced cache.InformerSynced

	numberOfRunningWorkers int
	workerCh               chan int
}

// NewShootController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <shootInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewShootController(k8sGardenClient kubernetes.Interface, k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory, kubeInformerFactory kubeinformers.SharedInformerFactory, config *config.ControllerManagerConfiguration, recorder record.EventRecorder) *Controller {
	var (
		gardenCoreV1beta1Informer = k8sGardenCoreInformers.Core().V1beta1()
		corev1Informer            = kubeInformerFactory.Core().V1()

		shootInformer = gardenCoreV1beta1Informer.Shoots()
		shootLister   = shootInformer.Lister()

		configMapInformer = corev1Informer.ConfigMaps()
		configMapLister   = configMapInformer.Lister()
	)

	shootController := &Controller{
		k8sGardenClient:        k8sGardenClient,
		k8sGardenCoreInformers: k8sGardenCoreInformers,

		config:                      config,
		recorder:                    recorder,
		maintenanceControl:          NewDefaultMaintenanceControl(k8sGardenClient, gardenCoreV1beta1Informer, recorder),
		quotaControl:                NewDefaultQuotaControl(k8sGardenClient, gardenCoreV1beta1Informer),
		hibernationScheduleRegistry: NewHibernationScheduleRegistry(),

		shootLister:     shootLister,
		configMapLister: configMapLister,

		shootMaintenanceQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-maintenance"),
		shootQuotaQueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-quota"),
		shootHibernationQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-hibernation"),
		configMapQueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "configmaps"),

		workerCh: make(chan int),
	}

	shootInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.shootMaintenanceAdd,
		UpdateFunc: shootController.shootMaintenanceUpdate,
		DeleteFunc: shootController.shootMaintenanceDelete,
	})

	shootInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.shootQuotaAdd,
		DeleteFunc: shootController.shootQuotaDelete,
	})

	shootInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.shootHibernationAdd,
		UpdateFunc: shootController.shootHibernationUpdate,
		DeleteFunc: shootController.shootHibernationDelete,
	})

	configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.configMapAdd,
		UpdateFunc: shootController.configMapUpdate,
	})

	shootController.shootSynced = shootInformer.Informer().HasSynced
	shootController.quotaSynced = gardenCoreV1beta1Informer.Quotas().Informer().HasSynced
	shootController.configMapSynced = configMapInformer.Informer().HasSynced

	return shootController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, shootMaintenanceWorkers, shootQuotaWorkers, shootHibernationWorkers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.shootSynced, c.quotaSynced, c.configMapSynced) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for {
			select {
			case res := <-c.workerCh:
				c.numberOfRunningWorkers += res
				logger.Logger.Debugf("Current number of running Shoot workers is %d", c.numberOfRunningWorkers)
			}
		}
	}()

	// Clean up orphaned `ShootState` resources for non-existing shoots (see https://github.com/gardener/gardener/pull/1855).
	// TODO: This code can be removed in a future version (post v1.0 release).
	shootList := &gardencorev1beta1.ShootList{}
	if err := c.k8sGardenClient.Client().List(ctx, shootList); err != nil {
		panic(fmt.Sprintf("Error listing Shoot resources: %+v", err))
	}

	shootMap := make(map[string]struct{}, len(shootList.Items))
	for _, shoot := range shootList.Items {
		key, err := cache.MetaNamespaceKeyFunc(&shoot)
		if err != nil {
			panic(fmt.Sprintf("Error constructing key for shoot %s/%s: %+v", shoot.Namespace, shoot.Name, err))
		}
		shootMap[key] = struct{}{}
	}

	shootStateList := &gardencorev1alpha1.ShootStateList{}
	if err := c.k8sGardenClient.Client().List(ctx, shootStateList); err != nil {
		panic(fmt.Sprintf("Error listing ShootState resources: %+v", err))
	}

	for _, shootState := range shootStateList.Items {
		key, err := cache.MetaNamespaceKeyFunc(&shootState)
		if err != nil {
			panic(fmt.Sprintf("Error constructing key for ShootState %s/%s: %+v", shootState.Namespace, shootState.Name, err))
		}

		if _, shootExists := shootMap[key]; !shootExists {
			logger.Logger.Infof("Deleting orphaned ShootState resource %s", key)
			obj := shootState.DeepCopy()
			if err := c.k8sGardenClient.Client().Delete(ctx, obj); client.IgnoreNotFound(err) != nil {
				panic(fmt.Sprintf("Error deleting orphaned ShootState %s/%s: %+v", shootState.Namespace, shootState.Name, err))
			}
		}
	}

	logger.Logger.Info("Shoot controller initialized.")

	for i := 0; i < shootMaintenanceWorkers; i++ {
		controllerutils.DeprecatedCreateWorker(ctx, c.shootMaintenanceQueue, "Shoot Maintenance", c.reconcileShootMaintenanceKey, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootQuotaWorkers; i++ {
		controllerutils.DeprecatedCreateWorker(ctx, c.shootQuotaQueue, "Shoot Quota", c.reconcileShootQuotaKey, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootHibernationWorkers; i++ {
		controllerutils.DeprecatedCreateWorker(ctx, c.shootHibernationQueue, "Scheduled Shoot Hibernation", c.reconcileShootHibernationKey, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootMaintenanceWorkers; i++ {
		controllerutils.DeprecatedCreateWorker(ctx, c.configMapQueue, "ConfigMap", c.reconcileConfigMapKey, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.shootMaintenanceQueue.ShutDown()
	c.shootQuotaQueue.ShutDown()
	c.shootHibernationQueue.ShutDown()
	c.configMapQueue.ShutDown()

	for {
		var (
			shootMaintenanceQueueLength = c.shootMaintenanceQueue.Len()
			shootQuotaQueueLength       = c.shootQuotaQueue.Len()
			shootHibernationQueueLength = c.shootHibernationQueue.Len()
			configMapQueueLength        = c.configMapQueue.Len()
			queueLengths                = shootMaintenanceQueueLength + shootQuotaQueueLength + shootHibernationQueueLength + configMapQueueLength
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
