// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package backupbucket

import (
	"context"
	"fmt"
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// ControllerName is the name of this controller.
	ControllerName = "backupbucket"
	// finalizerName is the backupbucket controller finalizer.
	finalizerName = "core.gardener.cloud/backupbucket"
)

// Controller controls BackupBuckets.
type Controller struct {
	log        logr.Logger
	reconciler reconcile.Reconciler

	backupBucketInformer runtimecache.Informer
	backupBucketQueue    workqueue.RateLimitingInterface

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewBackupBucketController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <backupBucketInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewBackupBucketController(
	ctx context.Context,
	log logr.Logger,
	gardenCluster cluster.Cluster,
	seedCluster cluster.Cluster,
	config *config.GardenletConfiguration,
) (
	*Controller,
	error,
) {
	log = log.WithName(ControllerName)

	backupBucketInformer, err := gardenCluster.GetCache().GetInformer(ctx, &gardencorev1beta1.BackupBucket{})
	if err != nil {
		return nil, fmt.Errorf("failed to get BackupBucket Informer: %w", err)
	}

	controller := &Controller{
		log:                  log,
		reconciler:           newReconciler(gardenCluster.GetClient(), seedCluster.GetClient(), gardenCluster.GetEventRecorderFor(ControllerName+"-controller"), config),
		backupBucketInformer: backupBucketInformer,
		backupBucketQueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "BackupBucket"),
		workerCh:             make(chan int),
	}

	controller.backupBucketInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.BackupBucketFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig)),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.backupBucketAdd,
			UpdateFunc: controller.backupBucketUpdate,
			DeleteFunc: controller.backupBucketDelete,
		},
	})

	return controller, nil
}

// Run runs the Controller until the given stop channel is closed.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.backupBucketInformer.HasSynced) {
		c.log.Error(wait.ErrWaitTimeout, "Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
		}
	}()

	c.log.Info("BackupBucket controller initialized")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.backupBucketQueue, "backupbucket", c.reconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log))
	}

	// Shutdown handling
	<-ctx.Done()
	c.backupBucketQueue.ShutDown()

	for {
		if c.backupBucketQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			c.log.V(1).Info("No running BackupBucket worker and no items left in the queues. Terminated BackupBucket controller")
			break
		}
		c.log.V(1).Info("Waiting for BackupBucket workers to finish", "numberOfRunningWorkers", c.numberOfRunningWorkers, "queueLength", c.backupBucketQueue.Len())
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}
