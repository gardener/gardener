// Copyright 2018 The Gardener Authors.
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
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/apis/componentconfig"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllerutils "github.com/gardener/gardener/pkg/controller/utils"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// Controller controls Shoots.
type Controller struct {
	k8sGardenClient    kubernetes.Client
	k8sGardenInformers gardeninformers.SharedInformerFactory

	config      *componentconfig.ControllerManagerConfiguration
	control     ControlInterface
	careControl CareControlInterface
	recorder    record.EventRecorder
	secrets     map[string]*corev1.Secret
	imageVector imagevector.ImageVector

	shootLister    gardenlisters.ShootLister
	shootQueue     workqueue.RateLimitingInterface
	shootCareQueue workqueue.RateLimitingInterface

	shootSynced                cache.InformerSynced
	seedSynced                 cache.InformerSynced
	cloudProfileSynced         cache.InformerSynced
	privateSecretBindingSynced cache.InformerSynced
	crossSecretBindingSynced   cache.InformerSynced

	numberOfRunningWorkers int
	workerCh               chan int
}

// NewShootController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <shootInformer>, and a <recorder> for
// event recording. It creates a new Garden controller.
func NewShootController(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.SharedInformerFactory, config *componentconfig.ControllerManagerConfiguration, identity *gardenv1beta1.Gardener, gardenNamespace string, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector, recorder record.EventRecorder) *Controller {
	var (
		gardenv1beta1Informer = k8sGardenInformers.Garden().V1beta1()
		shootInformer         = gardenv1beta1Informer.Shoots()
		shootLister           = shootInformer.Lister()
		shootUpdater          = NewRealUpdater(k8sGardenClient, shootLister)
	)

	shootController := &Controller{
		k8sGardenClient:    k8sGardenClient,
		k8sGardenInformers: k8sGardenInformers,
		config:             config,
		control:            NewDefaultControl(k8sGardenClient, gardenv1beta1Informer, secrets, imageVector, identity, config, gardenNamespace, recorder, shootUpdater),
		careControl:        NewDefaultCareControl(k8sGardenClient, gardenv1beta1Informer, secrets, imageVector, identity, config, shootUpdater),
		recorder:           recorder,
		secrets:            secrets,
		imageVector:        imageVector,
		shootLister:        shootLister,
		shootQueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot"),
		shootCareQueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-care"),
		workerCh:           make(chan int),
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

	shootController.shootSynced = shootInformer.Informer().HasSynced
	shootController.seedSynced = gardenv1beta1Informer.Seeds().Informer().HasSynced
	shootController.cloudProfileSynced = gardenv1beta1Informer.CloudProfiles().Informer().HasSynced
	shootController.privateSecretBindingSynced = gardenv1beta1Informer.PrivateSecretBindings().Informer().HasSynced
	shootController.crossSecretBindingSynced = gardenv1beta1Informer.CrossSecretBindings().Informer().HasSynced

	return shootController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	var (
		watchNamespace = c.config.Controller.WatchNamespace
		waitGroup      sync.WaitGroup
	)

	if !cache.WaitForCacheSync(stopCh, c.shootSynced, c.seedSynced, c.cloudProfileSynced, c.privateSecretBindingSynced, c.crossSecretBindingSynced) {
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

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(c.shootQueue, "Shoot", c.reconcileShootKey, stopCh, &waitGroup, c.workerCh)
		controllerutils.CreateWorker(c.shootCareQueue, "Shoot Care", c.reconcileShootCareKey, stopCh, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-stopCh
	c.shootQueue.ShutDown()
	c.shootCareQueue.ShutDown()

	for {
		var (
			shootQueueLength     = c.shootQueue.Len()
			shootCareQueueLength = c.shootCareQueue.Len()
			queueLengths         = shootQueueLength + shootCareQueueLength
		)
		if queueLengths == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Info("No running Shoot worker and no items left in the queues. Terminating Shoot controller...")
			break
		}
		logger.Logger.Infof("Waiting for %d Shoot worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, queueLengths)
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
		watchNamespace = c.config.Controller.WatchNamespace
	)
	return watchNamespace == nil || shoot.Namespace == *watchNamespace
}
