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

package managedseedset

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// Controller controls ManagedSeedSets.
type Controller struct {
	cache client.Reader
	log   logr.Logger

	reconciler reconcile.Reconciler

	managedSeedSetQueue workqueue.RateLimitingInterface

	numberOfRunningWorkers int
	workerCh               chan int
}

// NewManagedSeedSetController creates a new Gardener controller for ManagedSeedSets.
func NewManagedSeedSetController(
	ctx context.Context,
	log logr.Logger,
	mgr manager.Manager,
	config *config.ControllerManagerConfiguration,
) (*Controller, error) {
	log = log.WithName(ControllerName)

	gardenCache := mgr.GetCache()

	return &Controller{
		cache:               gardenCache,
		log:                 log,
		managedSeedSetQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ManagedSeedSet"),
		workerCh:            make(chan int),
	}, nil
}

// Run runs the Controller until the given context is cancelled.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done()) {
		c.log.Error(wait.ErrWaitTimeout, "Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
		}
	}()

	c.log.Info("ManagedSeedSet controller initialized")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.managedSeedSetQueue, "ManagedSeedSet", c.reconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log))
	}

	// Shutdown handling
	<-ctx.Done()
	c.managedSeedSetQueue.ShutDown()

	for {
		if c.managedSeedSetQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			c.log.V(1).Info("No running ManagedSeedSet worker and no items left in the queues. Terminating ManagedSeedSet controller")
			break
		}
		c.log.V(1).Info("Waiting for ManagedSeedSet workers to finish", "numberOfRunningWorkers", c.numberOfRunningWorkers, "queueLength", c.managedSeedSetQueue.Len())
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}
