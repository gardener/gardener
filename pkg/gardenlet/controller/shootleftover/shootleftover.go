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

package shootleftover

import (
	"context"
	"fmt"
	"sync"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller controls ShootLeftovers.
type Controller struct {
	gardenClient kubernetes.Interface
	clientMap    clientmap.ClientMap
	config       *config.GardenletConfiguration
	reconciler   reconcile.Reconciler

	shootLeftoverInformer runtimecache.Informer
	shootLeftoverQueue    workqueue.RateLimitingInterface

	numberOfRunningWorkers int
	workerCh               chan int

	logger logrus.FieldLogger
}

// NewShootLeftoverController creates a new Gardener controller for ShootLeftovers.
func NewShootLeftoverController(ctx context.Context, clientMap clientmap.ClientMap, config *config.GardenletConfiguration, recorder record.EventRecorder, logger logrus.FieldLogger) (*Controller, error) {
	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, fmt.Errorf("could not get garden client: %w", err)
	}

	shootLeftoverInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1alpha1.ShootLeftover{})
	if err != nil {
		return nil, fmt.Errorf("could not get ShootLeftover informer: %w", err)
	}

	actuator := newActuator(gardenClient, clientMap, logger)
	reconciler := newReconciler(gardenClient, actuator, config.Controllers.ShootLeftover, recorder, logger)

	return &Controller{
		gardenClient:          gardenClient,
		clientMap:             clientMap,
		config:                config,
		reconciler:            reconciler,
		shootLeftoverInformer: shootLeftoverInformer,
		shootLeftoverQueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ShootLeftover"),
		workerCh:              make(chan int),
		logger:                logger,
	}, nil
}

// Run runs the Controller until the given context is cancelled.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	c.shootLeftoverInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.ShootLeftoverFilterFunc(confighelper.SeedNameFromSeedConfig(c.config.SeedConfig)),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    c.shootLeftoverAdd,
			UpdateFunc: c.shootLeftoverUpdate,
			DeleteFunc: c.shootLeftoverDelete,
		},
	})

	if !cache.WaitForCacheSync(ctx.Done(), c.shootLeftoverInformer.HasSynced) {
		c.logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			c.logger.Debugf("Current number of running ShootLeftover workers is %d", c.numberOfRunningWorkers)
		}
	}()

	c.logger.Info("ShootLeftover controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.shootLeftoverQueue, "ShootLeftover", c.reconciler, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.shootLeftoverQueue.ShutDown()

	for {
		if c.shootLeftoverQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			c.logger.Debug("No running ShootLeftover worker and no items left in the queues. Terminated ShootLeftover controller...")
			break
		}
		c.logger.Debugf("Waiting for %d ShootLeftover worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.shootLeftoverQueue.Len())
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}
