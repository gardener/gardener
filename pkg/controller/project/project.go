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

package project

import (
	"context"
	"sync"
	"time"

	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllerutils "github.com/gardener/gardener/pkg/controller/utils"
	"github.com/gardener/gardener/pkg/logger"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// Controller controls Projects.
type Controller struct {
	k8sGardenClient    kubernetes.Client
	k8sGardenInformers gardeninformers.SharedInformerFactory

	k8sInformers kubeinformers.SharedInformerFactory

	control                 ControlInterface
	projectNamespaceControl ControlInterfaceProjectNamespace

	recorder record.EventRecorder

	projectLister gardenlisters.ProjectLister
	projectQueue  workqueue.RateLimitingInterface
	projectSynced cache.InformerSynced

	namespaceLister kubecorev1listers.NamespaceLister
	namespaceQueue  workqueue.RateLimitingInterface
	namespaceSynced cache.InformerSynced

	shootLister gardenlisters.ShootLister

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewProjectController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <projectInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewProjectController(k8sGardenClient kubernetes.Client, gardenInformerFactory gardeninformers.SharedInformerFactory, kubeInformerFactory kubeinformers.SharedInformerFactory, recorder record.EventRecorder) *Controller {
	var (
		gardenv1beta1Informer = gardenInformerFactory.Garden().V1beta1()
		corev1Informer        = kubeInformerFactory.Core().V1()

		projectInformer = gardenv1beta1Informer.Projects()
		projectLister   = projectInformer.Lister()

		namespaceInformer = corev1Informer.Namespaces()
		namespaceLister   = namespaceInformer.Lister()

		backupInfrastructureInformer = gardenv1beta1Informer.BackupInfrastructures()
		backupInfrastructureLister   = backupInfrastructureInformer.Lister()

		shootInformer = gardenv1beta1Informer.Shoots()
		shootLister   = shootInformer.Lister()

		projectUpdater = NewRealUpdater(k8sGardenClient, projectLister)
	)

	projectController := &Controller{
		k8sGardenClient:         k8sGardenClient,
		k8sGardenInformers:      gardenInformerFactory,
		control:                 NewDefaultControl(k8sGardenClient, gardenInformerFactory, recorder, projectUpdater, backupInfrastructureLister, shootLister, namespaceLister),
		projectNamespaceControl: NewDefaultProjectNamespaceControl(k8sGardenClient, kubeInformerFactory, namespaceLister, projectLister),
		recorder:                recorder,
		projectLister:           projectLister,
		projectQueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Project"),
		namespaceLister:         namespaceLister,
		namespaceQueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Namespace"),
		workerCh:                make(chan int),
	}

	projectInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    projectController.projectAdd,
		UpdateFunc: projectController.projectUpdate,
		DeleteFunc: projectController.projectDelete,
	})
	projectController.projectSynced = projectInformer.Informer().HasSynced

	namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    projectController.namespaceAdd,
		UpdateFunc: projectController.namespaceUpdate,
		DeleteFunc: projectController.namespaceDelete,
	})
	projectController.namespaceSynced = namespaceInformer.Informer().HasSynced

	return projectController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.projectSynced, c.namespaceSynced) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for {
			select {
			case res := <-c.workerCh:
				c.numberOfRunningWorkers += res
				logger.Logger.Debugf("Current number of running Project workers is %d", c.numberOfRunningWorkers)
			}
		}
	}()

	logger.Logger.Info("Project controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.projectQueue, "Project", c.reconcileProjectKey, &waitGroup, c.workerCh)
	}
	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.namespaceQueue, "Project Namespaces", c.reconcileProjectNamespaceKey, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.projectQueue.ShutDown()

	for {
		if c.projectQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running Project worker and no items left in the queues. Terminated Project controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d Project worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.projectQueue.Len())
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}

// RunningWorkers returns the number of running workers.
func (c *Controller) RunningWorkers() int {
	return c.numberOfRunningWorkers
}
