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

package privatesecretbinding

import (
	"sync"
	"time"

	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllerutils "github.com/gardener/gardener/pkg/controller/utils"
	"github.com/gardener/gardener/pkg/logger"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// Controller controls PrivateSecretBindings.
type Controller struct {
	k8sGardenClient    kubernetes.Client
	k8sGardenInformers gardeninformers.SharedInformerFactory

	k8sInformers kubeinformers.SharedInformerFactory

	control  ControlInterface
	recorder record.EventRecorder

	privateSecretBindingLister gardenlisters.PrivateSecretBindingLister
	privateSecretBindingQueue  workqueue.RateLimitingInterface
	privateSecretBindingSynced cache.InformerSynced

	shootLister gardenlisters.ShootLister

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewPrivateSecretBindingController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <privateSecretBindingInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewPrivateSecretBindingController(k8sGardenClient kubernetes.Client, gardenInformerFactory gardeninformers.SharedInformerFactory, kubeInformerFactory kubeinformers.SharedInformerFactory, recorder record.EventRecorder) *Controller {
	var (
		gardenv1beta1Informer = gardenInformerFactory.Garden().V1beta1()
		corev1Informer        = kubeInformerFactory.Core().V1()

		privateSecretBindingInformer = gardenv1beta1Informer.PrivateSecretBindings()
		privateSecretBindingLister   = privateSecretBindingInformer.Lister()
		secretLister                 = corev1Informer.Secrets().Lister()
		shootLister                  = gardenv1beta1Informer.Shoots().Lister()
	)

	privateSecretBindingController := &Controller{
		k8sGardenClient:            k8sGardenClient,
		k8sGardenInformers:         gardenInformerFactory,
		control:                    NewDefaultControl(k8sGardenClient, gardenInformerFactory, recorder, secretLister, shootLister),
		recorder:                   recorder,
		privateSecretBindingLister: privateSecretBindingLister,
		privateSecretBindingQueue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "PrivateSecretBinding"),
		shootLister:                shootLister,
		workerCh:                   make(chan int),
	}

	privateSecretBindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    privateSecretBindingController.privateSecretBindingAdd,
		UpdateFunc: privateSecretBindingController.privateSecretBindingUpdate,
		DeleteFunc: privateSecretBindingController.privateSecretBindingDelete,
	})
	privateSecretBindingController.privateSecretBindingSynced = privateSecretBindingInformer.Informer().HasSynced

	return privateSecretBindingController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(stopCh, c.privateSecretBindingSynced) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for {
			select {
			case res := <-c.workerCh:
				c.numberOfRunningWorkers += res
				logger.Logger.Debugf("Current number of running PrivateSecretBinding workers is %d", c.numberOfRunningWorkers)
			}
		}
	}()

	logger.Logger.Info("PrivateSecretBinding controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(c.privateSecretBindingQueue, "PrivateSecretBinding", c.reconcilePrivateSecretBindingKey, stopCh, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-stopCh
	c.privateSecretBindingQueue.ShutDown()

	for {
		if c.privateSecretBindingQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Info("No running PrivateSecretBinding worker and no items left in the queues. Terminating PrivateSecretBinding controller...")
			break
		}
		logger.Logger.Infof("Waiting for %d PrivateSecretBinding worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.privateSecretBindingQueue.Len())
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}

// RunningWorkers returns the number of running workers.
func (c *Controller) RunningWorkers() int {
	return c.numberOfRunningWorkers
}
