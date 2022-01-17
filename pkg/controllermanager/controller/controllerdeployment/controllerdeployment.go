// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controllerdeployment

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
)

const (
	// FinalizerName is the finalizer used by this controller.
	FinalizerName = "core.gardener.cloud/controllerdeployment"
	// ControllerName is the name of this controller.
	ControllerName = "controllerdeployment"
)

// Controller controls ManagedSeedSets.
type Controller struct {
	controllerDeploymentReconciler reconcile.Reconciler
	controllerDeploymentQueue      workqueue.RateLimitingInterface

	log                    logr.Logger
	hasSyncedFuncs         []cache.InformerSynced
	numberOfRunningWorkers int
	workerCh               chan int
}

// New creates a new Gardener controller for ControllerDeployments.
func New(
	ctx context.Context,
	log logr.Logger,
	clientMap clientmap.ClientMap,
) (*Controller, error) {
	log = log.WithName(ControllerName)

	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, fmt.Errorf("could not get garden client: %w", err)
	}

	controllerDeploymentInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.ControllerDeployment{})
	if err != nil {
		return nil, fmt.Errorf("could not get ControllerDeployment informer: %w", err)
	}

	controller := &Controller{
		controllerDeploymentReconciler: NewReconciler(gardenClient.Client()),
		controllerDeploymentQueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ManagedSeedSet"),
		log:                            log,
		workerCh:                       make(chan int),
	}

	controllerDeploymentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.controllerDeploymentAdd,
		UpdateFunc: controller.controllerDeploymentUpdate,
	})

	controller.hasSyncedFuncs = []cache.InformerSynced{
		controllerDeploymentInformer.HasSynced,
	}

	return controller, nil
}

// Run runs the Controller until the given context is cancelled.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		c.log.Error(wait.ErrWaitTimeout, "Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
		}
	}()

	c.log.Info("ControllerDeployment controller initialized")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.controllerDeploymentQueue, "ControllerDeployment", c.controllerDeploymentReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log))
	}

	// Shutdown handling
	<-ctx.Done()
	c.controllerDeploymentQueue.ShutDown()

	for {
		if c.controllerDeploymentQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			c.log.V(1).Info("No running ControllerDeployment worker and no items left in the queues. Terminating ControllerDeployment controller...")
			break
		}
		c.log.V(1).Info("Waiting for ControllerDeployment workers to finish...", "numberOfRunningWorkers", c.numberOfRunningWorkers, "queueLength", c.controllerDeploymentQueue.Len())
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}
