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
	"fmt"
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllermanager"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
	rbacv1 "k8s.io/api/rbac/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller controls Projects.
type Controller struct {
	gardenClient client.Client

	projectReconciler      reconcile.Reconciler
	projectStaleReconciler reconcile.Reconciler
	hasSyncedFuncs         []cache.InformerSynced

	projectQueue           workqueue.RateLimitingInterface
	projectStaleQueue      workqueue.RateLimitingInterface
	workerCh               chan int
	numberOfRunningWorkers int
}

// NewProjectController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <projectInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewProjectController(
	ctx context.Context,
	clientMap clientmap.ClientMap,
	gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	config *config.ControllerManagerConfiguration,
	recorder record.EventRecorder,
) (
	*Controller,
	error,
) {
	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, err
	}

	projectInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Project{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Shoot Informer: %w", err)
	}
	roleBindingInformer, err := gardenClient.Cache().GetInformer(ctx, &rbacv1.RoleBinding{})
	if err != nil {
		return nil, fmt.Errorf("failed to get RoleBinding Informer: %w", err)
	}

	var (
		gardenCoreV1beta1Informer = gardenCoreInformerFactory.Core().V1beta1()
		corev1Informer            = kubeInformerFactory.Core().V1()

		shootInformer = gardenCoreV1beta1Informer.Shoots()
		shootLister   = shootInformer.Lister()

		secretInformer = corev1Informer.Secrets()
		secretLister   = secretInformer.Lister()
	)

	projectController := &Controller{
		gardenClient:           gardenClient.Client(),
		projectReconciler:      NewProjectReconciler(logger.Logger, config.Controllers.Project, gardenClient, recorder),
		projectStaleReconciler: NewProjectStaleReconciler(logger.Logger, config.Controllers.Project, gardenClient.Client(), shootLister, secretLister),
		projectQueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Project"),
		projectStaleQueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Project Stale"),
		workerCh:               make(chan int),
	}

	projectInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    projectController.projectAdd,
		UpdateFunc: projectController.projectUpdate,
		DeleteFunc: projectController.projectDelete,
	})

	roleBindingInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) { projectController.roleBindingUpdate(ctx, oldObj, newObj) },
		DeleteFunc: func(obj interface{}) { projectController.roleBindingDelete(ctx, obj) },
	})

	projectController.hasSyncedFuncs = append(projectController.hasSyncedFuncs,
		projectInformer.HasSynced,
		roleBindingInformer.HasSynced,
	)

	return projectController, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
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
		controllerutils.CreateWorker(ctx, c.projectQueue, "Project", c.projectReconciler, &waitGroup, c.workerCh)
		controllerutils.CreateWorker(ctx, c.projectStaleQueue, "Project Stale", c.projectStaleReconciler, &waitGroup, c.workerCh)
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
