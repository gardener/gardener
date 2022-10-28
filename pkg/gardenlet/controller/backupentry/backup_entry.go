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

package backupentry

import (
	"context"
	"fmt"
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ControllerName is the name of this controller.
const ControllerName = "backupentry"

// Controller controls BackupEntries.
type Controller struct {
	log    logr.Logger
	config *config.GardenletConfiguration
	clock  clock.Clock

	reconciler          reconcile.Reconciler
	migrationReconciler reconcile.Reconciler

	backupEntryInformer       runtimecache.Informer
	seedInformer              runtimecache.Informer
	backupEntryMigrationQueue workqueue.RateLimitingInterface

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewBackupEntryController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <backupEntryInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewBackupEntryController(
	ctx context.Context,
	log logr.Logger,
	gardenCluster cluster.Cluster,
	seedCluster cluster.Cluster,
	config *config.GardenletConfiguration,
	clock clock.Clock,
	gardenNamespace string,
) (
	*Controller,
	error,
) {
	log = log.WithName(ControllerName)

	backupEntryInformer, err := gardenCluster.GetCache().GetInformer(ctx, &gardencorev1beta1.BackupEntry{})
	if err != nil {
		return nil, fmt.Errorf("failed to get BackupEntry Informer: %w", err)
	}

	seedInformer, err := gardenCluster.GetCache().GetInformer(ctx, &gardencorev1beta1.Seed{})
	if err != nil {
		return nil, fmt.Errorf("could not get Seed informer: %w", err)
	}

	controller := &Controller{
		log:                       log,
		config:                    config,
		clock:                     clock,
		migrationReconciler:       newMigrationReconciler(gardenCluster.GetClient(), config, clock),
		backupEntryInformer:       backupEntryInformer,
		seedInformer:              seedInformer,
		backupEntryMigrationQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "BackupEntryMigration"),
		workerCh:                  make(chan int),
	}

	if gardenletfeatures.FeatureGate.Enabled(features.ForceRestore) && confighelper.OwnerChecksEnabledInSeedConfig(config.SeedConfig) {
		controller.backupEntryInformer.AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controllerutils.BackupEntryMigrationFilterFunc(ctx, gardenCluster.GetCache(), confighelper.SeedNameFromSeedConfig(config.SeedConfig)),
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    controller.backupEntryMigrationAdd,
				UpdateFunc: controller.backupEntryMigrationUpdate,
				DeleteFunc: controller.backupEntryMigrationDelete,
			},
		})
	}

	return controller, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers, migrationWorkers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.backupEntryInformer.HasSynced, c.seedInformer.HasSynced) {
		c.log.Error(wait.ErrWaitTimeout, "Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
		}
	}()

	c.log.Info("BackupEntry controller initialized")

	if gardenletfeatures.FeatureGate.Enabled(features.ForceRestore) && confighelper.OwnerChecksEnabledInSeedConfig(c.config.SeedConfig) {
		for i := 0; i < migrationWorkers; i++ {
			controllerutils.CreateWorker(ctx, c.backupEntryMigrationQueue, "BackupEntry Migration", c.migrationReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(migrationReconcilerName)))
		}
	}

	// Shutdown handling
	<-ctx.Done()
	c.backupEntryMigrationQueue.ShutDown()

	for {
		queueLengths := c.backupEntryMigrationQueue.Len()
		if queueLengths == 0 && c.numberOfRunningWorkers == 0 {
			c.log.V(1).Info("No running BackupEntry worker and no items left in the queues. Terminated BackupEntry controller")
			break
		}
		c.log.V(1).Info("Waiting for BackupEntry workers to finish", "numberOfRunningWorkers", c.numberOfRunningWorkers, "queueLength", queueLengths)
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}
