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
	"fmt"
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/apis/componentconfig"
	garden "github.com/gardener/gardener/pkg/apis/garden"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllerutils "github.com/gardener/gardener/pkg/controller/utils"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/robfig/cron"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// Controller controls Shoots.
type Controller struct {
	k8sGardenClient    kubernetes.Client
	k8sGardenInformers gardeninformers.SharedInformerFactory

	config             *componentconfig.ControllerManagerConfiguration
	control            ControlInterface
	careControl        CareControlInterface
	maintenanceControl MaintenanceControlInterface
	quotaControl       QuotaControlInterface
	recorder           record.EventRecorder
	secrets            map[string]*corev1.Secret
	imageVector        imagevector.ImageVector

	shootLister           gardenlisters.ShootLister
	shootQueue            workqueue.RateLimitingInterface
	shootCareQueue        workqueue.RateLimitingInterface
	shootMaintenanceQueue workqueue.RateLimitingInterface
	shootQuotaQueue       workqueue.RateLimitingInterface
	shootSeedQueue        workqueue.RateLimitingInterface

	shootSynced         cache.InformerSynced
	seedSynced          cache.InformerSynced
	cloudProfileSynced  cache.InformerSynced
	secretBindingSynced cache.InformerSynced
	quotaSynced         cache.InformerSynced

	numberOfRunningWorkers int
	workerCh               chan int
}

// NewShootController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <shootInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewShootController(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.SharedInformerFactory, config *componentconfig.ControllerManagerConfiguration, identity *gardenv1beta1.Gardener, gardenNamespace string, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector, recorder record.EventRecorder) *Controller {
	var (
		gardenv1beta1Informer = k8sGardenInformers.Garden().V1beta1()
		shootInformer         = gardenv1beta1Informer.Shoots()
		shootLister           = shootInformer.Lister()
		shootUpdater          = NewRealUpdater(k8sGardenClient, shootLister)
	)

	shootController := &Controller{
		k8sGardenClient:       k8sGardenClient,
		k8sGardenInformers:    k8sGardenInformers,
		config:                config,
		control:               NewDefaultControl(k8sGardenClient, gardenv1beta1Informer, secrets, imageVector, identity, config, gardenNamespace, recorder, shootUpdater),
		careControl:           NewDefaultCareControl(k8sGardenClient, gardenv1beta1Informer, secrets, imageVector, identity, config, shootUpdater),
		maintenanceControl:    NewDefaultMaintenanceControl(k8sGardenClient, gardenv1beta1Informer, secrets, imageVector, identity, recorder, shootUpdater),
		quotaControl:          NewDefaultQuotaControl(k8sGardenClient, gardenv1beta1Informer),
		recorder:              recorder,
		secrets:               secrets,
		imageVector:           imageVector,
		shootLister:           shootLister,
		shootQueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot"),
		shootCareQueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-care"),
		shootMaintenanceQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-maintenance"),
		shootQuotaQueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-quota"),
		shootSeedQueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-seeds"),
		workerCh:              make(chan int),
	}

	shootInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: shootController.shootNamespaceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    shootController.shootAdd,
			UpdateFunc: shootController.shootUpdate,
			DeleteFunc: shootController.shootDelete,
		},
	})

	shootInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: shootController.shootNamespaceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    shootController.shootCareAdd,
			DeleteFunc: shootController.shootCareDelete,
		},
	})

	shootInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: shootController.shootNamespaceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    shootController.shootMaintenanceAdd,
			DeleteFunc: shootController.shootMaintenanceDelete,
		},
	})

	shootInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: shootController.shootNamespaceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    shootController.shootQuotaAdd,
			DeleteFunc: shootController.shootQuotaDelete,
		},
	})

	shootController.shootSynced = shootInformer.Informer().HasSynced
	shootController.seedSynced = gardenv1beta1Informer.Seeds().Informer().HasSynced
	shootController.cloudProfileSynced = gardenv1beta1Informer.CloudProfiles().Informer().HasSynced
	shootController.secretBindingSynced = gardenv1beta1Informer.SecretBindings().Informer().HasSynced
	shootController.quotaSynced = gardenv1beta1Informer.Quotas().Informer().HasSynced

	return shootController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(shootWorkers, shootCareWorkers, shootMaintenanceWorkers, shootQuotaWorkers int, stopCh <-chan struct{}) {
	var (
		watchNamespace = c.config.Controllers.Shoot.WatchNamespace
		waitGroup      sync.WaitGroup
	)

	if !cache.WaitForCacheSync(stopCh, c.shootSynced, c.seedSynced, c.cloudProfileSynced, c.secretBindingSynced, c.quotaSynced) {
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

	if watchNamespace == nil {
		logger.Logger.Info("Watching all namespaces for Shoot resources...")
	} else {
		logger.Logger.Infof("Watching only namespace '%s' for Shoot resources...", *watchNamespace)
	}
	logger.Logger.Info("Shoot controller initialized.")

	// Update Shoots before starting the workers.
	shoots, err := c.shootLister.List(labels.Everything())
	if err != nil {
		logger.Logger.Errorf("Failed to fetch shoots resources: %v", err.Error())
		return
	}
	for _, shoot := range shoots {
		var (
			newShoot    = shoot.DeepCopy()
			needsUpdate = false
		)

		// Check if the backup defaults are valid. If not, update the shoots with the default backup schedule.
		schedule, err := cron.ParseStandard(shoot.Spec.Backup.Schedule)
		if err != nil {
			logger.Logger.Errorf("Failed to parse the schedule for shoot [%v]: %v ", shoot.ObjectMeta.Name, err.Error())
			return
		}

		var (
			nextScheduleTime              = schedule.Next(time.Now())
			scheduleTimeAfterNextSchedule = schedule.Next(nextScheduleTime)
			granularity                   = scheduleTimeAfterNextSchedule.Sub(nextScheduleTime)
		)

		if shoot.DeletionTimestamp == nil && granularity < garden.MinimumETCDFullBackupTimeInterval {
			newShoot.Spec.Backup.Schedule = garden.DefaultETCDBackupSchedule
			needsUpdate = true
		}

		// Check if the status indicates that an operation is processing and mark it as "aborted".
		if shoot.Status.LastOperation != nil && shoot.Status.LastOperation.State == gardenv1beta1.ShootLastOperationStateProcessing {
			newShoot.Status.LastOperation.State = gardenv1beta1.ShootLastOperationStateAborted
			needsUpdate = true
		}

		if needsUpdate {
			if _, err := c.k8sGardenClient.GardenClientset().Garden().Shoots(newShoot.Namespace).Update(newShoot); err != nil {
				panic(fmt.Sprintf("Failed to update shoot [%v]: %v ", newShoot.ObjectMeta.Name, err.Error()))
			}
		}
	}

	for i := 0; i < shootWorkers; i++ {
		controllerutils.CreateWorker(c.shootQueue, "Shoot", c.reconcileShootKey, stopCh, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootCareWorkers; i++ {
		controllerutils.CreateWorker(c.shootCareQueue, "Shoot Care", c.reconcileShootCareKey, stopCh, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootMaintenanceWorkers; i++ {
		controllerutils.CreateWorker(c.shootMaintenanceQueue, "Shoot Maintenance", c.reconcileShootMaintenanceKey, stopCh, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootQuotaWorkers; i++ {
		controllerutils.CreateWorker(c.shootQuotaQueue, "Shoot Quota", c.reconcileShootQuotaKey, stopCh, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootWorkers/2+1; i++ {
		controllerutils.CreateWorker(c.shootSeedQueue, "Shooted Seeds", c.reconcileShootKey, stopCh, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-stopCh
	c.shootQueue.ShutDown()
	c.shootCareQueue.ShutDown()
	c.shootMaintenanceQueue.ShutDown()
	c.shootQuotaQueue.ShutDown()
	c.shootSeedQueue.ShutDown()

	for {
		var (
			shootQueueLength            = c.shootQueue.Len()
			shootCareQueueLength        = c.shootCareQueue.Len()
			shootMaintenanceQueueLength = c.shootMaintenanceQueue.Len()
			shootQuotaQueueLength       = c.shootQuotaQueue.Len()
			shootSeedQueueLength        = c.shootSeedQueue.Len()
			queueLengths                = shootQueueLength + shootCareQueueLength + shootMaintenanceQueueLength + shootQuotaQueueLength + shootSeedQueueLength
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

// shootNamespaceFilter filters Shoots based on their namespace and the configuration value.
func (c *Controller) shootNamespaceFilter(obj interface{}) bool {
	var (
		shoot          = obj.(*gardenv1beta1.Shoot)
		watchNamespace = c.config.Controllers.Shoot.WatchNamespace
	)
	return watchNamespace == nil || shoot.Namespace == *watchNamespace
}

func (c *Controller) getShootQueue(obj interface{}) workqueue.RateLimitingInterface {
	if shoot, ok := obj.(*gardenv1beta1.Shoot); ok {
		if shootUsedAsSeed, _, _ := helper.IsUsedAsSeed(shoot); shootUsedAsSeed {
			return c.shootSeedQueue
		}
	}
	return c.shootQueue
}
