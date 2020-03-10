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

	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// Controller controls Projects.
type Controller struct {
	k8sGardenClient        kubernetes.Interface
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory

	control  ControlInterface
	recorder record.EventRecorder

	projectLister gardencorelisters.ProjectLister
	projectQueue  workqueue.RateLimitingInterface
	projectSynced cache.InformerSynced

	namespaceLister kubecorev1listers.NamespaceLister
	namespaceSynced cache.InformerSynced

	rolebindingSynced cache.InformerSynced

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewProjectController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <projectInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewProjectController(k8sGardenClient kubernetes.Interface, gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory, kubeInformerFactory kubeinformers.SharedInformerFactory, recorder record.EventRecorder) *Controller {
	var (
		gardenCoreV1beta1Informer = gardenCoreInformerFactory.Core().V1beta1()
		corev1Informer            = kubeInformerFactory.Core().V1()
		rbacv1Informer            = kubeInformerFactory.Rbac().V1()

		projectInformer = gardenCoreV1beta1Informer.Projects()
		projectLister   = projectInformer.Lister()

		namespaceInformer = corev1Informer.Namespaces()
		namespaceLister   = namespaceInformer.Lister()

		rolebindingInformer = rbacv1Informer.RoleBindings()

		projectUpdater = NewRealUpdater(k8sGardenClient, projectLister)
	)

	projectController := &Controller{
		k8sGardenClient:        k8sGardenClient,
		k8sGardenCoreInformers: gardenCoreInformerFactory,
		control:                NewDefaultControl(k8sGardenClient, gardenCoreInformerFactory, recorder, projectUpdater, namespaceLister),
		recorder:               recorder,
		projectLister:          projectLister,
		projectQueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Project"),
		namespaceLister:        namespaceLister,
		workerCh:               make(chan int),
	}

	projectInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    projectController.projectAdd,
		UpdateFunc: projectController.projectUpdate,
		DeleteFunc: projectController.projectDelete,
	})

	rolebindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: projectController.rolebindingUpdate,
		DeleteFunc: projectController.rolebindingDelete,
	})

	projectController.projectSynced = projectInformer.Informer().HasSynced
	projectController.namespaceSynced = namespaceInformer.Informer().HasSynced
	projectController.rolebindingSynced = rolebindingInformer.Informer().HasSynced

	return projectController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.projectSynced, c.namespaceSynced, c.rolebindingSynced) {
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
		controllerutils.DeprecatedCreateWorker(ctx, c.projectQueue, "Project", c.reconcileProjectKey, &waitGroup, c.workerCh)
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

// CollectMetrics implements gardenmetrics.ControllerMetricsCollector interface
func (c *Controller) CollectMetrics(ch chan<- prometheus.Metric) {
	metric, err := prometheus.NewConstMetric(controllermanager.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "project")
	if err != nil {
		controllermanager.ScrapeFailures.With(prometheus.Labels{"kind": "project-controller"}).Inc()
		return
	}
	ch <- metric
}
