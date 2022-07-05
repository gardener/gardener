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

package bastion

import (
	"context"
	"fmt"
	"sync"
	"time"

	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// ControllerName is the name of this controller.
	ControllerName = "bastion-controller"
)

// Controller controls Bastions.
type Controller struct {
	log        logr.Logger
	reconciler reconcile.Reconciler

	hasSyncedFuncs []cache.InformerSynced
	bastionQueue   workqueue.RateLimitingInterface

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewBastionController takes a context <ctx>, a map of Kubernetes clients for for both the
// garden and seed clusters <clientMap> and the gardenlet configuration to extract the config
// for itself <config>. It creates a new Gardener controller.
func NewBastionController(
	ctx context.Context,
	log logr.Logger,
	clientMap clientmap.ClientMap,
	config *config.GardenletConfiguration,
) (
	*Controller,
	error,
) {
	log = log.WithName(ControllerName)

	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, fmt.Errorf("failed to get garden client: %w", err)
	}

	bastionInformer, err := gardenClient.Cache().GetInformer(ctx, &operationsv1alpha1.Bastion{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Bastion Informer: %w", err)
	}

	controller := &Controller{
		log:          log,
		reconciler:   newReconciler(clientMap, config),
		bastionQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Bastion"),
		workerCh:     make(chan int),
		hasSyncedFuncs: []cache.InformerSynced{
			bastionInformer.HasSynced,
		},
	}

	bastionInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.BastionFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig)),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.bastionAdd,
			UpdateFunc: controller.bastionUpdate,
			DeleteFunc: controller.bastionDelete,
		},
	})

	return controller, nil
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

	c.log.Info("Bastion controller initialized")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.bastionQueue, "bastion", c.reconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log))
	}

	// Shutdown handling
	<-ctx.Done()
	c.bastionQueue.ShutDown()

	for {
		if c.bastionQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			c.log.V(1).Info("No running Bastion worker and no items left in the queues. Terminated Bastion controller")
			break
		}
		c.log.V(1).Info("Waiting for Bastion workers to finish", "numberOfRunningWorkers", c.numberOfRunningWorkers, "queueLength", c.bastionQueue.Len())
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}

func (c *Controller) bastionAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.log.Error(err, "Could not get key", "obj", obj)
		return
	}

	c.bastionQueue.Add(key)
}

func (c *Controller) bastionUpdate(_, newObj interface{}) {
	newBastion := newObj.(*operationsv1alpha1.Bastion)

	// If the generation did not change for an update event (i.e., no changes to the .spec section have
	// been made), we do not want to add the Bastion to the queue. The periodic reconciliation is handled
	// elsewhere by adding the Bastion to the queue to dedicated times.
	if newBastion.Status.ObservedGeneration != nil && newBastion.Generation == *newBastion.Status.ObservedGeneration {
		return
	}

	c.bastionAdd(newObj)
}

func (c *Controller) bastionDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		c.log.Error(err, "Could not get key", "obj", obj)
		return
	}
	c.bastionQueue.Add(key)
}

// IsBastionManagedByThisGardenlet checks if the given Bastion is managed by this gardenlet by comparing it with the seed name from the GardenletConfiguration.
func IsBastionManagedByThisGardenlet(bastion *operationsv1alpha1.Bastion, gc *config.GardenletConfiguration) bool {
	seedName := confighelper.SeedNameFromSeedConfig(gc.SeedConfig)
	return bastion.Spec.SeedName != nil && *bastion.Spec.SeedName == seedName
}
