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

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	controllerutils "github.com/gardener/gardener/pkg/controllermanager/controller/utils"
	gardenmetrics "github.com/gardener/gardener/pkg/controllermanager/metrics"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/reconcilescheduler"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/robfig/cron"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// Controller controls Shoots.
type Controller struct {
	k8sGardenClient    kubernetes.Interface
	k8sGardenInformers gardeninformers.SharedInformerFactory

	config                 *config.ControllerManagerConfiguration
	control                ControlInterface
	careControl            CareControlInterface
	maintenanceControl     MaintenanceControlInterface
	quotaControl           QuotaControlInterface
	recorder               record.EventRecorder
	secrets                map[string]*corev1.Secret
	imageVector            imagevector.ImageVector
	scheduler              reconcilescheduler.Interface
	shootToHibernationCron map[string]*cron.Cron

	seedLister      gardenlisters.SeedLister
	shootLister     gardenlisters.ShootLister
	projectLister   gardenlisters.ProjectLister
	namespaceLister kubecorev1listers.NamespaceLister
	configMapLister kubecorev1listers.ConfigMapLister

	seedQueue             workqueue.RateLimitingInterface
	shootQueue            workqueue.RateLimitingInterface
	shootCareQueue        workqueue.RateLimitingInterface
	shootMaintenanceQueue workqueue.RateLimitingInterface
	shootQuotaQueue       workqueue.RateLimitingInterface
	shootSeedQueue        workqueue.RateLimitingInterface
	configMapQueue        workqueue.RateLimitingInterface
	shootHibernationQueue workqueue.RateLimitingInterface

	shootSynced         cache.InformerSynced
	seedSynced          cache.InformerSynced
	cloudProfileSynced  cache.InformerSynced
	secretBindingSynced cache.InformerSynced
	quotaSynced         cache.InformerSynced
	projectSynced       cache.InformerSynced
	namespaceSynced     cache.InformerSynced
	configMapSynced     cache.InformerSynced

	numberOfRunningWorkers int
	workerCh               chan int
}

// NewShootController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <shootInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewShootController(k8sGardenClient kubernetes.Interface, k8sGardenInformers gardeninformers.SharedInformerFactory, kubeInformerFactory kubeinformers.SharedInformerFactory, config *config.ControllerManagerConfiguration, identity *gardenv1beta1.Gardener, gardenNamespace string, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector, recorder record.EventRecorder) *Controller {
	var (
		gardenv1beta1Informer = k8sGardenInformers.Garden().V1beta1()
		corev1Informer        = kubeInformerFactory.Core().V1()

		shootInformer = gardenv1beta1Informer.Shoots()
		shootLister   = shootInformer.Lister()

		seedInformer = gardenv1beta1Informer.Seeds()
		seedLister   = seedInformer.Lister()

		projectInformer = gardenv1beta1Informer.Projects()
		projectLister   = projectInformer.Lister()

		namespaceInformer = corev1Informer.Namespaces()
		namespaceLister   = namespaceInformer.Lister()

		configMapInformer = corev1Informer.ConfigMaps()
		configMapLister   = configMapInformer.Lister()
	)

	shootController := &Controller{
		k8sGardenClient:    k8sGardenClient,
		k8sGardenInformers: k8sGardenInformers,

		config:                 config,
		control:                NewDefaultControl(k8sGardenClient, gardenv1beta1Informer, secrets, imageVector, identity, config, gardenNamespace, recorder),
		careControl:            NewDefaultCareControl(k8sGardenClient, gardenv1beta1Informer, secrets, imageVector, identity, config),
		maintenanceControl:     NewDefaultMaintenanceControl(k8sGardenClient, gardenv1beta1Informer, secrets, imageVector, identity, recorder),
		quotaControl:           NewDefaultQuotaControl(k8sGardenClient, gardenv1beta1Informer),
		recorder:               recorder,
		secrets:                secrets,
		imageVector:            imageVector,
		scheduler:              reconcilescheduler.New(nil),
		shootToHibernationCron: make(map[string]*cron.Cron),

		seedLister:      seedLister,
		shootLister:     shootLister,
		projectLister:   projectLister,
		namespaceLister: namespaceLister,
		configMapLister: configMapLister,

		seedQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "seed"),
		shootQueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot"),
		shootCareQueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-care"),
		shootMaintenanceQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-maintenance"),
		shootQuotaQueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-quota"),
		shootSeedQueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-seeds"),
		configMapQueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "configMap"),
		shootHibernationQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-hibernation"),

		workerCh: make(chan int),
	}

	seedInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.seedAdd,
		UpdateFunc: shootController.seedUpdate,
		DeleteFunc: shootController.seedDelete,
	})

	shootInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.shootAdd,
		UpdateFunc: shootController.shootUpdate,
		DeleteFunc: shootController.shootDelete,
	})

	shootInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: shootController.shootCareAdd,
	})

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

	shootController.seedSynced = seedInformer.Informer().HasSynced
	shootController.shootSynced = shootInformer.Informer().HasSynced
	shootController.cloudProfileSynced = gardenv1beta1Informer.CloudProfiles().Informer().HasSynced
	shootController.secretBindingSynced = gardenv1beta1Informer.SecretBindings().Informer().HasSynced
	shootController.quotaSynced = gardenv1beta1Informer.Quotas().Informer().HasSynced
	shootController.projectSynced = projectInformer.Informer().HasSynced
	shootController.namespaceSynced = namespaceInformer.Informer().HasSynced
	shootController.configMapSynced = configMapInformer.Informer().HasSynced

	return shootController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, shootWorkers, shootCareWorkers, shootMaintenanceWorkers, shootQuotaWorkers, shootHibernationWorkers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.shootSynced, c.seedSynced, c.cloudProfileSynced, c.secretBindingSynced, c.quotaSynced, c.projectSynced, c.namespaceSynced, c.configMapSynced) {
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

	// Update Shoots before starting the workers.
	shoots, err := c.shootLister.List(labels.Everything())
	if err != nil {
		logger.Logger.Errorf("Failed to fetch shoots resources: %v", err.Error())
		return
	}
	for _, shoot := range shoots {
		newShoot := shoot.DeepCopy()

		// Check if the status indicates that an operation is processing and mark it as "aborted".
		if shoot.Status.LastOperation != nil && shoot.Status.LastOperation.State == gardenv1beta1.ShootLastOperationStateProcessing {
			newShoot.Status.LastOperation.State = gardenv1beta1.ShootLastOperationStateAborted
			if _, err := c.k8sGardenClient.Garden().Garden().Shoots(newShoot.Namespace).UpdateStatus(newShoot); err != nil {
				panic(fmt.Sprintf("Failed to update shoot status [%v]: %v ", newShoot.Name, err.Error()))
			}
		}
	}

	logger.Logger.Info("Shoot controller initialized.")

	for i := 0; i < shootWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.shootQueue, "Shoot", c.reconcileShootKey, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootCareWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.shootCareQueue, "Shoot Care", c.reconcileShootCareKey, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootMaintenanceWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.shootMaintenanceQueue, "Shoot Maintenance", c.reconcileShootMaintenanceKey, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootQuotaWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.shootQuotaQueue, "Shoot Quota", c.reconcileShootQuotaKey, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootWorkers/2+1; i++ {
		controllerutils.CreateWorker(ctx, c.shootSeedQueue, "Shooted Seeds", c.reconcileShootKey, &waitGroup, c.workerCh)
		controllerutils.CreateWorker(ctx, c.seedQueue, "Seed Queue", c.reconcileSeedKey, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootWorkers/5+1; i++ {
		controllerutils.CreateWorker(ctx, c.configMapQueue, "ConfigMap", c.reconcileConfigMapKey, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootHibernationWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.shootHibernationQueue, "Scheduled Shoot Hibernation", c.reconcileShootHibernationKey, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.shootQueue.ShutDown()
	c.shootCareQueue.ShutDown()
	c.shootMaintenanceQueue.ShutDown()
	c.shootQuotaQueue.ShutDown()
	c.shootSeedQueue.ShutDown()
	c.configMapQueue.ShutDown()
	c.shootHibernationQueue.ShutDown()

	for {
		var (
			shootQueueLength            = c.shootQueue.Len()
			shootCareQueueLength        = c.shootCareQueue.Len()
			shootMaintenanceQueueLength = c.shootMaintenanceQueue.Len()
			shootQuotaQueueLength       = c.shootQuotaQueue.Len()
			shootSeedQueueLength        = c.shootSeedQueue.Len()
			seedQueueLength             = c.seedQueue.Len()
			configMapQueueLength        = c.configMapQueue.Len()
			shootHibernationQueueLength = c.shootHibernationQueue.Len()
			queueLengths                = shootQueueLength + shootCareQueueLength + shootMaintenanceQueueLength + shootQuotaQueueLength + shootSeedQueueLength + seedQueueLength + configMapQueueLength + shootHibernationQueueLength
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
	metric, err := prometheus.NewConstMetric(gardenmetrics.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "shoot")
	if err != nil {
		gardenmetrics.ScrapeFailures.With(prometheus.Labels{"kind": "shoot-controller"}).Inc()
		return
	}
	ch <- metric
}

func (c *Controller) getShootQueue(obj interface{}) workqueue.RateLimitingInterface {
	if shoot, ok := obj.(*gardenv1beta1.Shoot); ok && shootIsSeed(shoot) {
		return c.shootSeedQueue
	}
	return c.shootQueue
}
