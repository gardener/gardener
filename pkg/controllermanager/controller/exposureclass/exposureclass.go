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

package exposureclass

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
)

const (
	// ControllerName is the name of this controller.
	ControllerName = "exposureclass"
)

// Controller controls ExposureClasses.
type Controller struct {
	reconciler     reconcile.Reconciler
	hasSyncedFuncs []cache.InformerSynced

	log                    logr.Logger
	exposureClassQueue     workqueue.RateLimitingInterface
	workerCh               chan int
	numberOfRunningWorkers int
}

// NewExposureClassController receives a Kubernetes client <k8sGardenClient> and a <k8sGardenCoreInformers> for the Garden clusters.
// It creates and return a new Garden controller to control ExposureClasses.
func NewExposureClassController(
	ctx context.Context,
	log logr.Logger,
	clientMap clientmap.ClientMap,
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

	exposureClassInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1alpha1.ExposureClass{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ExposureClass Informer: %w", err)
	}

	exposureClassController := &Controller{
		reconciler:         NewExposureClassReconciler(gardenClient.Client(), recorder),
		log:                log,
		exposureClassQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "exposureclass"),
		workerCh:           make(chan int),
	}

	exposureClassInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    exposureClassController.exposureClassAdd,
		UpdateFunc: exposureClassController.exposureClassUpdate,
		DeleteFunc: exposureClassController.exposureClassDelete,
	})

	exposureClassController.hasSyncedFuncs = append(exposureClassController.hasSyncedFuncs, exposureClassInformer.HasSynced)

	return exposureClassController, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	// Check if informers cache has been populated.
	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		c.log.Error(wait.ErrWaitTimeout, "Timed out waiting for caches to sync")
		return
	}

	go func() {
		for x := range c.workerCh {
			c.numberOfRunningWorkers += x
		}
	}()

	c.log.Info("ExposureClass controller initialized")

	// Start the workers
	var waitGroup sync.WaitGroup
	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.exposureClassQueue, "ExposureClass", c.reconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log))
	}

	<-ctx.Done()
	c.exposureClassQueue.ShutDown()

	for {
		if c.exposureClassQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			c.log.V(1).Info("No running ExposureClass worker and no items left in the queues. Terminating ExposureClass controller")
			break
		}
		c.log.V(1).Info("Waiting for ExposureClass workers to finish", "numberOfRunningWorkers", c.numberOfRunningWorkers, "queueLength", c.exposureClassQueue.Len())
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}
