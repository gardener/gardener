// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package extensions

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ControllerName is the name of this controller.
const ControllerName = "extensions"

// Controller watches the extension resources and has several control loops.
type Controller struct {
	log logr.Logger

	initialized            bool
	waitGroup              sync.WaitGroup
	workerCh               chan int
	numberOfRunningWorkers int

	controllerArtifacts controllerArtifacts
}

// NewController creates new controller that syncs extensions states to ShootState
func NewController(
	log logr.Logger,
	gardenCluster cluster.Cluster,
	seedCluster cluster.Cluster,
	seedName string,
) *Controller {
	log = log.WithName(ControllerName)

	controller := &Controller{
		log:      log,
		workerCh: make(chan int),

		controllerArtifacts: newControllerArtifacts(),
	}

	return controller
}

// Initialize sets up all necessary dependencies to run this controller.
// This function must be called before Run is executed.
func (c *Controller) Initialize(ctx context.Context, seedCluster cluster.Cluster) error {
	if err := c.controllerArtifacts.initialize(ctx, seedCluster); err != nil {
		return err
	}
	c.initialized = true
	return nil
}

// Run creates workers that reconciles extension resources.
// Initialize must be called before running the controller.
func (c *Controller) Run(ctx context.Context) {
	if !c.initialized {
		panic("Extensions controller is not initialized, cannot run it")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute*2)
	defer cancel()

	if !cache.WaitForCacheSync(timeoutCtx.Done(), c.controllerArtifacts.hasSyncedFuncs...) {
		c.log.Error(wait.ErrWaitTimeout, "Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
		}
	}()

	c.log.Info("Extension controller initialized")
}

// Stop the controller
func (c *Controller) Stop() {
	c.controllerArtifacts.shutdownQueues()
	c.waitGroup.Wait()
}

// createEnqueueEmptyRequestFunc creates a func that enqueues an empty reconcile.Request into the given queue, which
// can serve as a trigger for a reconciler that actually doesn't care about which object was created/deleted, but
// only about that some object of a given kind was created/deleted.
func createEnqueueEmptyRequestFunc(queue workqueue.RateLimitingInterface) func(extensionObject interface{}) {
	return func(_ interface{}) {
		queue.Add(reconcile.Request{})
	}
}

// createEnqueueEmptyRequestOnUpdateFunc is similar to createEnqueueEmptyRequestFunc in that it enqueues an empty
// reconcile.Request, but it only does it if an update matched the given predicateFunc.
func createEnqueueEmptyRequestOnUpdateFunc(queue workqueue.RateLimitingInterface, predicateFunc func(new, old interface{}) bool) func(newExtensionObject, oldExtensionObject interface{}) {
	return func(newObj, oldObj interface{}) {
		if predicateFunc != nil && !predicateFunc(newObj, oldObj) {
			return
		}

		queue.Add(reconcile.Request{})
	}
}
