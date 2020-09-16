// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"sync"
	"time"

	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
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
)

// Controller controls Shoots.
type Controller struct {
	clientMap              clientmap.ClientMap
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

// NewShootController takes a ClientMap, a GardenerInformerFactory, a KubernetesInformerFactory, a
// ControllerManagerConfig struct and an EventRecorder to create a new Shoot controller.
func NewShootController(clientMap clientmap.ClientMap, k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory, kubeInformerFactory kubeinformers.SharedInformerFactory, config *config.ControllerManagerConfiguration, recorder record.EventRecorder) *Controller {
	var (
		gardenCoreV1beta1Informer = k8sGardenCoreInformers.Core().V1beta1()
		corev1Informer            = kubeInformerFactory.Core().V1()

		shootInformer = gardenCoreV1beta1Informer.Shoots()
		shootLister   = shootInformer.Lister()

		configMapInformer = corev1Informer.ConfigMaps()
		configMapLister   = configMapInformer.Lister()
	)

	shootController := &Controller{
		clientMap:              clientMap,
		k8sGardenCoreInformers: k8sGardenCoreInformers,

		config:                      config,
		recorder:                    recorder,
		maintenanceControl:          NewDefaultMaintenanceControl(clientMap, gardenCoreV1beta1Informer, config.Controllers.ShootMaintenance, recorder),
		quotaControl:                NewDefaultQuotaControl(clientMap, gardenCoreV1beta1Informer),
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
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running Shoot workers is %d", c.numberOfRunningWorkers)
		}
	}()

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
