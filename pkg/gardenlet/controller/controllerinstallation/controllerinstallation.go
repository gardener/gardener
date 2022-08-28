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

package controllerinstallation

import (
	"context"
	"fmt"
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	finalizerName = "core.gardener.cloud/controllerinstallation"

	// ControllerName is the name of this controller.
	ControllerName = "controllerinstallation"
)

// Controller controls ControllerInstallation.
type Controller struct {
	log logr.Logger

	reconciler     reconcile.Reconciler
	careReconciler reconcile.Reconciler

	controllerInstallationQueue     workqueue.RateLimitingInterface
	controllerInstallationCareQueue workqueue.RateLimitingInterface

	hasSyncedFuncs         []cache.InformerSynced
	workerCh               chan int
	numberOfRunningWorkers int
}

// NewController instantiates a new ControllerInstallation controller.
func NewController(
	ctx context.Context,
	log logr.Logger,
	clientMap clientmap.ClientMap,
	config *config.GardenletConfiguration,
	identity *gardencorev1beta1.Gardener,
	gardenNamespace *corev1.Namespace,
	gardenClusterIdentity string,
) (
	*Controller,
	error,
) {
	log = log.WithName(ControllerName)

	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, err
	}

	seedClient, err := clientMap.GetClient(ctx, keys.ForSeedWithName(config.SeedConfig.Name))
	if err != nil {
		return nil, err
	}

	controllerInstallationInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.ControllerInstallation{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ControllerInstallation Informer: %w", err)
	}

	controller := &Controller{
		log: log,

		reconciler:     newReconciler(clientMap, identity, gardenNamespace, gardenClusterIdentity),
		careReconciler: NewCareReconciler(gardenClient.Client(), seedClient.Client(), *config.Controllers.ControllerInstallationCare),

		controllerInstallationQueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "controllerinstallation"),
		controllerInstallationCareQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "controllerinstallation-care"),

		workerCh: make(chan int),
	}

	controllerInstallationInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.ControllerInstallationFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig)),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.controllerInstallationAdd,
			UpdateFunc: controller.controllerInstallationUpdate,
			DeleteFunc: controller.controllerInstallationDelete,
		},
	})

	// TODO: add a watch for ManagedResources and run the care reconciler on changed to the MR conditions
	controllerInstallationInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.ControllerInstallationFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig)),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: controller.controllerInstallationCareAdd,
		},
	})

	controller.hasSyncedFuncs = []cache.InformerSynced{
		controllerInstallationInformer.HasSynced,
	}

	return controller, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers, careWorkers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		c.log.Error(wait.ErrWaitTimeout, "Timed out waiting for caches to sync")
		return
	}

	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
		}
	}()

	c.log.Info("ControllerInstallation controller initialized")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.controllerInstallationQueue, "ControllerInstallation", c.reconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(reconcilerName)))
	}

	for i := 0; i < careWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.controllerInstallationCareQueue, "ControllerInstallation Care", c.careReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(careReconcilerName)))
	}

	// Shutdown handling
	<-ctx.Done()
	c.controllerInstallationQueue.ShutDown()
	c.controllerInstallationCareQueue.ShutDown()

	for {
		if c.controllerInstallationQueue.Len() == 0 && c.controllerInstallationCareQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			c.log.V(1).Info("No running ControllerInstallation worker and no items left in the queues. Terminated ControllerInstallation controller")

			break
		}
		c.log.V(1).Info("Waiting for ControllerInstallation workers to finish", "numberOfRunningWorkers", c.numberOfRunningWorkers, "queueLength", c.controllerInstallationQueue.Len()+c.controllerInstallationCareQueue.Len())
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}
