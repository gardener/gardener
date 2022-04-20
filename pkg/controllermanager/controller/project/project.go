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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ControllerName is the name of this controller.
const ControllerName = "project"

// Controller controls Projects.
type Controller struct {
	gardenClient client.Client
	log          logr.Logger

	projectReconciler                    reconcile.Reconciler
	projectStaleReconciler               reconcile.Reconciler
	projectShootActivityReconciler       reconcile.Reconciler
	projectSecretActivityReconciler      reconcile.Reconciler
	projectPlantActivityReconciler       reconcile.Reconciler
	projectBackupEntryActivityReconciler reconcile.Reconciler
	projectQuotaActivityReconciler       reconcile.Reconciler

	hasSyncedFuncs []cache.InformerSynced

	projectQueue                    workqueue.RateLimitingInterface
	projectStaleQueue               workqueue.RateLimitingInterface
	projectShootActivityQueue       workqueue.RateLimitingInterface
	projectSecretActivityQueue      workqueue.RateLimitingInterface
	projectPlantActivityQueue       workqueue.RateLimitingInterface
	projectBackupEntryActivityQueue workqueue.RateLimitingInterface
	projectQuotaActivityQueue       workqueue.RateLimitingInterface

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewProjectController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <projectInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewProjectController(
	ctx context.Context,
	log logr.Logger,
	clientMap clientmap.ClientMap,
	config *config.ControllerManagerConfiguration,
	recorder record.EventRecorder,
) (
	*Controller,
	error,
) {
	log = log.WithName(ControllerName)

	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, err
	}

	projectInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Project{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Project Informer: %w", err)
	}
	roleBindingInformer, err := gardenClient.Cache().GetInformer(ctx, &rbacv1.RoleBinding{})
	if err != nil {
		return nil, fmt.Errorf("failed to get RoleBinding Informer: %w", err)
	}
	shootInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Shoot{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Shoot Informer: %w", err)
	}
	secretInformer, err := gardenClient.Cache().GetInformer(ctx, &corev1.Secret{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Secret Informer: %w", err)
	}
	plantInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Plant{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Plant Informer: %w", err)
	}
	backupEntryInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.BackupEntry{})
	if err != nil {
		return nil, fmt.Errorf("failed to get BackupEntry Informer: %w", err)
	}
	quotaInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Quota{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Quota Informer: %w", err)
	}

	projectController := &Controller{
		gardenClient:                         gardenClient.Client(),
		log:                                  log,
		projectReconciler:                    NewProjectReconciler(config.Controllers.Project, gardenClient, recorder),
		projectStaleReconciler:               NewProjectStaleReconciler(config.Controllers.Project, gardenClient.Client()),
		projectShootActivityReconciler:       NewShootActivityReconciler(gardenClient.Client()),
		projectSecretActivityReconciler:      NewSecretActivityReconciler(gardenClient.Client()),
		projectPlantActivityReconciler:       NewPlantActivityReconciler(gardenClient.Client()),
		projectBackupEntryActivityReconciler: NewBackupEntryActivityReconciler(gardenClient.Client()),
		projectQuotaActivityReconciler:       NewQuotaActivityReconciler(gardenClient.Client()),
		projectQueue:                         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Project"),
		projectStaleQueue:                    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Project Stale"),
		projectShootActivityQueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Project Shoot Activity"),
		projectSecretActivityQueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Project Secret Activity"),
		projectPlantActivityQueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Project Plant Activity"),
		projectBackupEntryActivityQueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Project BackupEntry Activity"),
		projectQuotaActivityQueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Project Quota Activity"),
		workerCh:                             make(chan int),
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

	shootInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: projectController.updateShootActivity,
	})

	secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: projectController.updateSecretActivity,
	})

	plantInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: projectController.updatePlantActivity,
	})

	backupEntryInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: projectController.updateBackupEntryActivity,
	})

	quotaInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: projectController.updateQuotaActivity,
	})

	projectController.hasSyncedFuncs = append(projectController.hasSyncedFuncs,
		projectInformer.HasSynced,
		roleBindingInformer.HasSynced,
		shootInformer.HasSynced,
		secretInformer.HasSynced,
		plantInformer.HasSynced,
		backupEntryInformer.HasSynced,
		quotaInformer.HasSynced,
	)

	return projectController, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		c.log.Error(wait.ErrWaitTimeout, "Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
		}
	}()

	c.log.Info("Project controller initialized")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.projectQueue, "Project", c.projectReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(projectReconcilerName)))
		controllerutils.CreateWorker(ctx, c.projectStaleQueue, "Project Stale", c.projectStaleReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(staleReconcilerName)))
		controllerutils.CreateWorker(ctx, c.projectShootActivityQueue, "Project Shoot Activity", c.projectShootActivityReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(shootActivityReconcilerName)))
		controllerutils.CreateWorker(ctx, c.projectSecretActivityQueue, "Project Secret Activity", c.projectSecretActivityReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(secretActivityReconcilerName)))
		controllerutils.CreateWorker(ctx, c.projectPlantActivityQueue, "Project Plant Activity", c.projectPlantActivityReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(plantActivityReconcilerName)))
		controllerutils.CreateWorker(ctx, c.projectBackupEntryActivityQueue, "Project BackupEntry Activity", c.projectBackupEntryActivityReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(backupEntryActivityReconcilerName)))
		controllerutils.CreateWorker(ctx, c.projectQuotaActivityQueue, "Project Quota Activity", c.projectQuotaActivityReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(quotaActivityReconcilerName)))
	}

	// Shutdown handling
	<-ctx.Done()
	c.projectQueue.ShutDown()
	c.projectStaleQueue.ShutDown()
	c.projectShootActivityQueue.ShutDown()
	c.projectSecretActivityQueue.ShutDown()
	c.projectPlantActivityQueue.ShutDown()
	c.projectBackupEntryActivityQueue.ShutDown()
	c.projectQuotaActivityQueue.ShutDown()

	for {
		if c.projectQueue.Len() == 0 &&
			c.projectStaleQueue.Len() == 0 &&
			c.projectShootActivityQueue.Len() == 0 &&
			c.projectSecretActivityQueue.Len() == 0 &&
			c.projectPlantActivityQueue.Len() == 0 &&
			c.projectBackupEntryActivityQueue.Len() == 0 &&
			c.projectQuotaActivityQueue.Len() == 0 &&
			c.numberOfRunningWorkers == 0 {
			c.log.V(1).Info("No running Project worker and no items left in the queues. Terminating Project controller")
			break
		}
		c.log.V(1).Info(
			"Waiting for Project workers to finish",
			"numberOfRunningWorkers", c.numberOfRunningWorkers,
			"queueLength", c.projectQueue.Len()+c.projectStaleQueue.Len()+c.projectShootActivityQueue.Len()+c.projectSecretActivityQueue.Len()+c.projectPlantActivityQueue.Len()+c.projectBackupEntryActivityQueue.Len()+c.projectQuotaActivityQueue.Len(),
		)
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}
