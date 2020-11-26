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

// Controller controls Projects.
type Controller struct {
	clientMap              clientmap.ClientMap
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory

	control      ControlInterface
	staleControl StaleControlInterface

	config   *config.ControllerManagerConfiguration
	recorder record.EventRecorder

	projectLister     gardencorelisters.ProjectLister
	projectQueue      workqueue.RateLimitingInterface
	projectStaleQueue workqueue.RateLimitingInterface
	projectSynced     cache.InformerSynced

	namespaceLister kubecorev1listers.NamespaceLister
	namespaceSynced cache.InformerSynced

	roleBindingSynced cache.InformerSynced

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewProjectController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <projectInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewProjectController(clientMap clientmap.ClientMap, gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory, kubeInformerFactory kubeinformers.SharedInformerFactory, config *config.ControllerManagerConfiguration, recorder record.EventRecorder) *Controller {
	var (
		gardenCoreV1beta1Informer = gardenCoreInformerFactory.Core().V1beta1()
		corev1Informer            = kubeInformerFactory.Core().V1()
		rbacv1Informer            = kubeInformerFactory.Rbac().V1()

		projectInformer = gardenCoreV1beta1Informer.Projects()
		projectLister   = projectInformer.Lister()

		shootInformer = gardenCoreV1beta1Informer.Shoots()
		shootLister   = shootInformer.Lister()

		plantInformer = gardenCoreV1beta1Informer.Plants()
		plantLister   = plantInformer.Lister()

		backupEntryInformer = gardenCoreV1beta1Informer.BackupEntries()
		backupEntryLister   = backupEntryInformer.Lister()

		secretBindingInformer = gardenCoreV1beta1Informer.SecretBindings()
		secretBindingLister   = secretBindingInformer.Lister()

		quotaInformer = gardenCoreV1beta1Informer.Quotas()
		quotaLister   = quotaInformer.Lister()

		namespaceInformer = corev1Informer.Namespaces()
		namespaceLister   = namespaceInformer.Lister()

		secretInformer = corev1Informer.Secrets()
		secretLister   = secretInformer.Lister()

		roleBindingInformer = rbacv1Informer.RoleBindings()
	)

	projectController := &Controller{
		clientMap:              clientMap,
		k8sGardenCoreInformers: gardenCoreInformerFactory,
		control:                NewDefaultControl(clientMap, config, gardenCoreInformerFactory, recorder, namespaceLister),
		staleControl:           NewDefaultStaleControl(clientMap, config, shootLister, plantLister, backupEntryLister, secretBindingLister, quotaLister, namespaceLister, secretLister),
		config:                 config,
		recorder:               recorder,
		projectLister:          projectLister,
		projectQueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Project"),
		projectStaleQueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Project Stale"),
		namespaceLister:        namespaceLister,
		workerCh:               make(chan int),
	}

	projectInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    projectController.projectAdd,
		UpdateFunc: projectController.projectUpdate,
		DeleteFunc: projectController.projectDelete,
	})

	roleBindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: projectController.roleBindingUpdate,
		DeleteFunc: projectController.roleBindingDelete,
	})

	projectController.projectSynced = projectInformer.Informer().HasSynced
	projectController.namespaceSynced = namespaceInformer.Informer().HasSynced
	projectController.roleBindingSynced = roleBindingInformer.Informer().HasSynced

	return projectController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.projectSynced, c.namespaceSynced, c.roleBindingSynced) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running Project workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("Project controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.DeprecatedCreateWorker(ctx, c.projectQueue, "Project", c.reconcileProjectKey, &waitGroup, c.workerCh)
		controllerutils.DeprecatedCreateWorker(ctx, c.projectStaleQueue, "Project Stale", c.reconcileStaleProjectKey, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.projectQueue.ShutDown()
	c.projectStaleQueue.ShutDown()

	for {
		if c.projectQueue.Len() == 0 && c.projectStaleQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running Project worker and no items left in the queues. Terminated Project controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d Project worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.projectQueue.Len()+c.projectStaleQueue.Len())
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
