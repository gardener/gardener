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

package secretbinding

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
	"github.com/gardener/gardener/pkg/controllerutils"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ControllerName is the name of this controller.
const ControllerName = "secretbinding"

// Controller controls SecretBindings.
type Controller struct {
	reconciler                      reconcile.Reconciler
	secretBindingProviderReconciler reconcile.Reconciler

	secretBindingQueue workqueue.RateLimitingInterface
	shootQueue         workqueue.RateLimitingInterface

	log                    logr.Logger
	hasSyncedFuncs         []cache.InformerSynced
	workerCh               chan int
	numberOfRunningWorkers int
}

// NewSecretBindingController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <secretBindingInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewSecretBindingController(
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

	secretBindingInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.SecretBinding{})
	if err != nil {
		return nil, fmt.Errorf("failed to get SecretBinding Informer: %w", err)
	}

	shootInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Shoot{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Shoot Informer: %w", err)
	}

	secretBindingController := &Controller{
		reconciler:                      NewSecretBindingReconciler(gardenClient.Client(), recorder),
		secretBindingProviderReconciler: NewSecretBindingProviderReconciler(gardenClient.Client()),
		secretBindingQueue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "SecretBinding"),
		shootQueue:                      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Shoot"),
		log:                             log,
		workerCh:                        make(chan int),
	}

	secretBindingInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    secretBindingController.secretBindingAdd,
		UpdateFunc: secretBindingController.secretBindingUpdate,
		DeleteFunc: secretBindingController.secretBindingDelete,
	})

	shootInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: secretBindingController.shootAdd,
	})

	secretBindingController.hasSyncedFuncs = append(secretBindingController.hasSyncedFuncs, secretBindingInformer.HasSynced, shootInformer.HasSynced)

	return secretBindingController, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, secretBindingWorkers, secretBindingProviderWorkers int) {
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

	c.log.Info("SecretBinding controller initialized")

	for i := 0; i < secretBindingWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.secretBindingQueue, "SecretBinding", c.reconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log))
	}
	for i := 0; i < secretBindingProviderWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.shootQueue, "SecretBinding Provider", c.secretBindingProviderReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(providerTypeReconcilerName)))
	}

	// Shutdown handling
	<-ctx.Done()
	c.secretBindingQueue.ShutDown()
	c.shootQueue.ShutDown()

	for {
		queueLengths := c.secretBindingQueue.Len() + c.shootQueue.Len()
		if queueLengths == 0 && c.numberOfRunningWorkers == 0 {
			c.log.V(1).Info("No running SecretBinding worker and no items left in the queues. Terminating SecretBinding controller")
			break
		}
		c.log.V(1).Info("Waiting for SecretBinding workers to finish", "numberOfRunningWorkers", c.numberOfRunningWorkers, "queueLength", queueLengths)
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}
