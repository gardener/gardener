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

package seed

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/clock"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ControllerName is the name of this controller.
const ControllerName = "seed"

// Controller controls Seeds.
type Controller struct {
	gardenClient client.Client
	config       *config.ControllerManagerConfiguration
	log          logr.Logger

	lifeCycleReconciler reconcile.Reconciler

	seedLifecycleQueue workqueue.RateLimitingInterface

	hasSyncedFuncs         []cache.InformerSynced
	workerCh               chan int
	numberOfRunningWorkers int
}

// NewSeedController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <gardenInformerFactory>, and a <recorder> for
// event recording. It creates a new Seed controller.
func NewSeedController(
	ctx context.Context,
	log logr.Logger,
	mgr manager.Manager,
	config *config.ControllerManagerConfiguration,
) (
	*Controller,
	error,
) {
	log = log.WithName(ControllerName)

	gardenClient := mgr.GetClient()
	gardenCache := mgr.GetCache()

	seedInformer, err := gardenCache.GetInformer(ctx, &gardencorev1beta1.Seed{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Seed Informer: %w", err)
	}

	seedController := &Controller{
		gardenClient: gardenClient,
		config:       config,
		log:          log,

		lifeCycleReconciler: NewLifecycleReconciler(gardenClient, clock.RealClock{}, config),

		seedLifecycleQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Seed Lifecycle"),
		workerCh:           make(chan int),
	}

	seedInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: seedController.seedLifecycleAdd,
	})

	seedController.hasSyncedFuncs = []cache.InformerSynced{
		seedInformer.HasSynced,
	}

	return seedController, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, seedWorkers int) {
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

	c.log.Info("Seed controller initialized")

	var waitGroup sync.WaitGroup
	for i := 0; i < seedWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.seedLifecycleQueue, "Seed Lifecycle", c.lifeCycleReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(seedLifecycleReconcilerName)))
	}

	// Shutdown handling
	<-ctx.Done()
	c.seedLifecycleQueue.ShutDown()

	for {
		queueLength := c.seedLifecycleQueue.Len()
		if queueLength == 0 && c.numberOfRunningWorkers == 0 {
			c.log.V(1).Info("No running Seed worker and no items left in the queues. Terminating Seed controller")
			break
		}
		c.log.V(1).Info("Waiting for Seed workers to finish", "numberOfRunningWorkers", c.numberOfRunningWorkers, "queueLength", queueLength)
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}
