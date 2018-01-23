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

package seed

import (
	"sync"
	"time"

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

// Controller controls Seeds.
type Controller struct {
	k8sGardenClient    kubernetes.Client
	k8sGardenInformers gardeninformers.SharedInformerFactory

	control  ControlInterface
	recorder record.EventRecorder

	seedLister gardenlisters.SeedLister
	seedQueue  workqueue.RateLimitingInterface
	seedSynced cache.InformerSynced

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewSeedController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <seedInformer>, and a <recorder> for
// event recording. It creates a new Garden controller.
func NewSeedController(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.SharedInformerFactory, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector, recorder record.EventRecorder) *Controller {
	var (
		gardenv1beta1Informer = k8sGardenInformers.Garden().V1beta1()
		seedInformer          = gardenv1beta1Informer.Seeds()
		seedLister            = seedInformer.Lister()
		seedUpdater           = NewRealUpdater(k8sGardenClient, seedLister)
	)

	seedController := &Controller{
		k8sGardenClient:    k8sGardenClient,
		k8sGardenInformers: k8sGardenInformers,
		control:            NewDefaultControl(k8sGardenClient, k8sGardenInformers, secrets, imageVector, recorder, seedUpdater),
		recorder:           recorder,
		seedLister:         seedLister,
		seedQueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "seed"),
		workerCh:           make(chan int),
	}

	seedInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    seedController.seedAdd,
		UpdateFunc: seedController.seedUpdate,
		DeleteFunc: seedController.seedDelete,
	})
	seedController.seedSynced = seedInformer.Informer().HasSynced

	return seedController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(stopCh, c.seedSynced) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for {
			select {
			case res := <-c.workerCh:
				c.numberOfRunningWorkers += res
				logger.Logger.Debugf("Current number of running Seed workers is %d", c.numberOfRunningWorkers)
			}
		}
	}()

	logger.Logger.Info("Seed controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(c.seedQueue, "Seed", c.reconcileSeedKey, stopCh, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-stopCh
	c.seedQueue.ShutDown()

	for {
		if c.seedQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Info("No running Seed worker and no items left in the queues. Terminating Seed controller...")
			break
		}
		logger.Logger.Infof("Waiting for %d Seed worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.seedQueue.Len())
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}

// RunningWorkers returns the number of running workers.
func (c *Controller) RunningWorkers() int {
	return c.numberOfRunningWorkers
}
