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

package controllerregistration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

const (
	// ControllerName is the name of this controller.
	ControllerName = "controllerregistration"
)

// Controller implements the logic behind ControllerRegistrations. It creates and deletes ControllerInstallations for
// ControllerRegistrations for the Seeds where they are needed or not.
type Controller struct {
	gardenClient client.Client
	log          logr.Logger

	hasSyncedFuncs         []cache.InformerSynced
	workerCh               chan int
	numberOfRunningWorkers int
}

// NewController instantiates a new ControllerRegistration controller.
func NewController(ctx context.Context, log logr.Logger, mgr manager.Manager) (*Controller, error) {
	log = log.WithName(ControllerName)

	gardenClient := mgr.GetClient()
	gardenCache := mgr.GetCache()

	backupBucketInformer, err := gardenCache.GetInformer(ctx, &gardencorev1beta1.BackupBucket{})
	if err != nil {
		return nil, fmt.Errorf("failed to get BackupBucket Informer: %w", err)
	}
	backupEntryInformer, err := gardenCache.GetInformer(ctx, &gardencorev1beta1.BackupEntry{})
	if err != nil {
		return nil, fmt.Errorf("failed to get BackupEntry Informer: %w", err)
	}
	controllerDeploymentInformer, err := gardenCache.GetInformer(ctx, &gardencorev1beta1.ControllerDeployment{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ControllerDeployment Informer: %w", err)
	}
	controllerInstallationInformer, err := gardenCache.GetInformer(ctx, &gardencorev1beta1.ControllerInstallation{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ControllerInstallation Informer: %w", err)
	}
	shootInformer, err := gardenCache.GetInformer(ctx, &gardencorev1beta1.Shoot{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Shoot Informer: %w", err)
	}

	controller := &Controller{
		gardenClient: gardenClient,
		log:          log,

		workerCh: make(chan int),
	}

	backupBucketInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.backupBucketAdd,
		UpdateFunc: controller.backupBucketUpdate,
		DeleteFunc: controller.backupBucketDelete,
	})

	backupEntryInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.backupEntryAdd,
		UpdateFunc: controller.backupEntryUpdate,
		DeleteFunc: controller.backupEntryDelete,
	})

	controllerDeploymentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { controller.controllerDeploymentAdd(ctx, obj) },
		UpdateFunc: func(oldObj, newObj interface{}) { controller.controllerDeploymentUpdate(ctx, oldObj, newObj) },
	})

	controllerInstallationInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.controllerInstallationAdd,
		UpdateFunc: controller.controllerInstallationUpdate,
	})

	shootInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.shootAdd,
		UpdateFunc: controller.shootUpdate,
		DeleteFunc: controller.shootDelete,
	})

	controller.hasSyncedFuncs = append(controller.hasSyncedFuncs,
		backupBucketInformer.HasSynced,
		backupEntryInformer.HasSynced,
		controllerDeploymentInformer.HasSynced,
		controllerInstallationInformer.HasSynced,
		shootInformer.HasSynced,
	)

	return controller, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
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

	c.log.Info("ControllerRegistration controller initialized")

	// Shutdown handling
	<-ctx.Done()

	for {
		if c.numberOfRunningWorkers == 0 {
			c.log.V(1).Info("No running ControllerRegistration worker and no items left in the queues. Terminating ControllerRegistration controller")
			break
		}
		c.log.V(1).Info("Waiting for ControllerRegistration workers to finish", "numberOfRunningWorkers", c.numberOfRunningWorkers)
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}

func (c *Controller) enqueueAllSeeds(ctx context.Context) {
	seedList := &metav1.PartialObjectMetadataList{}
	seedList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("SeedList"))
	if err := c.gardenClient.List(ctx, seedList); err != nil {
		return
	}

	for _, seed := range seedList.Items {
		c.seedQueue.Add(seed.Name)
	}
}
