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
	"os"
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/apis/componentconfig"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controller"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/version"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// Controller controls Shoots.
type Controller struct {
	k8sGardenClient            kubernetes.Client
	k8sGardenInformers         gardeninformers.Interface
	config                     *componentconfig.ControllerManagerConfiguration
	control                    ControlInterface
	careControl                CareControlInterface
	recorder                   record.EventRecorder
	secrets                    map[string]*corev1.Secret
	shootLister                gardenlisters.ShootLister
	shootQueue                 workqueue.RateLimitingInterface
	shootCareQueue             workqueue.RateLimitingInterface
	shootSynced                cache.InformerSynced
	seedSynced                 cache.InformerSynced
	cloudProfileSynced         cache.InformerSynced
	privateSecretBindingSynced cache.InformerSynced
	crossSecretBindingSynced   cache.InformerSynced
	workerCh                   chan int
}

// NewShootController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <shootInformer>, and a <recorder> for
// event recording. It creates a new Garden controller.
func NewShootController(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.Interface, config *componentconfig.ControllerManagerConfiguration, identity *gardenv1beta1.Gardener, gardenerNamespace string, recorder record.EventRecorder) *Controller {
	var (
		shootInformer = k8sGardenInformers.Shoots()
		shootLister   = shootInformer.Lister()
		shootUpdater  = NewRealUpdater(k8sGardenClient, shootLister)
	)

	secrets, err := garden.ReadGardenSecrets(k8sGardenClient, config.GardenNamespace, config.ClientConnection.KubeConfigFile == "")
	if err != nil {
		panic(err)
	}

	shootController := &Controller{
		k8sGardenClient:    k8sGardenClient,
		k8sGardenInformers: k8sGardenInformers,
		config:             config,
		control:            NewDefaultControl(k8sGardenClient, k8sGardenInformers, secrets, identity, config, gardenerNamespace, recorder, shootUpdater),
		careControl:        NewDefaultCareControl(k8sGardenClient, k8sGardenInformers, secrets, identity, config, shootUpdater),
		recorder:           recorder,
		secrets:            secrets,
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
	shootController.seedSynced = k8sGardenInformers.Seeds().Informer().HasSynced
	shootController.cloudProfileSynced = k8sGardenInformers.CloudProfiles().Informer().HasSynced
	shootController.privateSecretBindingSynced = k8sGardenInformers.PrivateSecretBindings().Informer().HasSynced
	shootController.crossSecretBindingSynced = k8sGardenInformers.CrossSecretBindings().Informer().HasSynced

	return shootController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	var (
		numberOfRunningWorkers = 0
		watchNamespace         = c.config.Controller.WatchNamespace
		waitGroup              sync.WaitGroup
	)

	if !cache.WaitForCacheSync(stopCh, c.shootSynced, c.seedSynced, c.cloudProfileSynced, c.privateSecretBindingSynced, c.crossSecretBindingSynced) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	if err := garden.BootstrapCluster(c.k8sGardenClient, c.config.GardenNamespace, c.secrets); err != nil {
		logger.Logger.Errorf("Failed to bootstrap the Garden cluster: %s", err.Error())
		return
	}
	logger.Logger.Info("Successfully bootstrapped the Garden cluster.")
	if err := seed.BootstrapClusters(c.k8sGardenClient, c.k8sGardenInformers, c.secrets); err != nil {
		logger.Logger.Errorf("Failed to bootstrap the Seed clusters: %s", err.Error())
		return
	}
	logger.Logger.Info("Successfully bootstrapped the Seed clusters.")

	if watchNamespace == nil {
		logger.Logger.Info("Watching all namespaces for Shoot resources...")
	} else {
		logger.Logger.Infof("Watching only namespace '%s' for Shoot resources...", *watchNamespace)
	}
	logger.Logger.Infof("Garden controller manager (version %s) initialized successfully.", version.Version)

	// This Goroutine implements the counting of running workers by checking the workerCh
	// channel. The received value should be 1 or -1 in order to increment/decrement the worker
	// counter. The value "-1" should be sent as soon as a worker has been completed (independent
	// of success or failure).
	go func() {
		for {
			select {
			case res := <-c.workerCh:
				numberOfRunningWorkers += res
				logger.Logger.Debugf("Current number of running workers is %d", numberOfRunningWorkers)
			}
		}
	}()

	for i := 0; i < workers; i++ {
		controller.CreateWorker(c.shootQueue, "Shoot", c.reconcileShootKey, stopCh, &waitGroup, c.workerCh)
		controller.CreateWorker(c.shootCareQueue, "Shoot Care", c.reconcileShootCareKey, stopCh, &waitGroup, c.workerCh)
	}

	<-stopCh
	logger.Logger.Info("I have received a stop signal and will no longer watch events of my API group.")
	logger.Logger.Info("I will terminate as soon as all my running workers have come to an end.")

	c.shootQueue.ShutDown()
	c.shootCareQueue.ShutDown()

	for {
		var (
			shootQueueLength     = c.shootQueue.Len()
			shootCareQueueLength = c.shootCareQueue.Len()
			queueLengths         = shootQueueLength + shootCareQueueLength
		)
		if queueLengths == 0 && numberOfRunningWorkers == 0 {
			logger.Logger.Info("No running worker and no items left in the queues. Exiting...")
			break
		}
		logger.Logger.Infof("Waiting for %d worker(s) to finish (%d item(s) left in the queues)...", numberOfRunningWorkers, queueLengths)
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
	os.Exit(0)
}

// shootNamespaceFilter filters Shoots based on their namespace and the configuration value.
func (c *Controller) shootNamespaceFilter(obj interface{}) bool {
	var (
		shoot          = obj.(*gardenv1beta1.Shoot)
		watchNamespace = c.config.Controller.WatchNamespace
	)
	return watchNamespace == nil || shoot.ObjectMeta.Namespace == *watchNamespace
}
