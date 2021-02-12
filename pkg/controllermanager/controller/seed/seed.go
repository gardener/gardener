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

package seed

import (
	"context"
	"sync"
	"time"

	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/controllermanager"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller controls Seeds.
type Controller struct {
	clientMap              clientmap.ClientMap
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory

	config              *config.ControllerManagerConfiguration
	lifeCycleReconciler reconcile.Reconciler
	recorder            record.EventRecorder

	seedBackupReconciler reconcile.Reconciler

	backupBucketLister    gardencorelisters.BackupBucketLister
	seedBackupBucketQueue workqueue.RateLimitingInterface

	seedLister         gardencorelisters.SeedLister
	seedQueue          workqueue.RateLimitingInterface
	seedLifecycleQueue workqueue.RateLimitingInterface

	shootLister gardencorelisters.ShootLister

	hasSyncedFuncs         []cache.InformerSynced
	workerCh               chan int
	numberOfRunningWorkers int
}

// NewSeedController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <gardenInformerFactory>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewSeedController(
	clientMap clientmap.ClientMap,
	gardenInformerFactory gardencoreinformers.SharedInformerFactory,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	config *config.ControllerManagerConfiguration,
	recorder record.EventRecorder,
) *Controller {
	var (
		gardenCoreV1beta1Informer = gardenInformerFactory.Core().V1beta1()

		backupBucketInformer = gardenCoreV1beta1Informer.BackupBuckets()
		backupBucketLister   = backupBucketInformer.Lister()

		seedInformer = gardenCoreV1beta1Informer.Seeds()
		seedLister   = seedInformer.Lister()

		shootInformer = gardenCoreV1beta1Informer.Shoots()
		shootLister   = shootInformer.Lister()

		leaseInformer = kubeInformerFactory.Coordination().V1().Leases()
		leaseLister   = leaseInformer.Lister()
	)

	seedController := &Controller{
		clientMap:              clientMap,
		k8sGardenCoreInformers: gardenInformerFactory,
		config:                 config,
		lifeCycleReconciler:    NewLifecycleDefaultControl(clientMap, leaseLister, seedLister, shootLister, config),
		recorder:               recorder,
		seedBackupReconciler:   NewDefaultBackupBucketControl(clientMap, backupBucketLister, seedLister),
		backupBucketLister:     backupBucketLister,
		seedBackupBucketQueue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Backup Bucket"),
		seedLister:             seedLister,
		shootLister:            shootLister,
		seedLifecycleQueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Seed Lifecycle"),
		seedQueue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Seed"),
		workerCh:               make(chan int),
	}

	backupBucketInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    seedController.backupBucketAdd,
		UpdateFunc: seedController.backupBucketUpdate,
	})

	seedInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: seedController.seedLifecycleAdd,
	})

	seedController.hasSyncedFuncs = []cache.InformerSynced{
		backupBucketInformer.Informer().HasSynced,
		seedInformer.Informer().HasSynced,
		shootInformer.Informer().HasSynced,
		leaseInformer.Informer().HasSynced,
	}

	return seedController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running Seed workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("Seed controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.seedLifecycleQueue, "Seed Lifecycle", c.lifeCycleReconciler, &waitGroup, c.workerCh)
		controllerutils.CreateWorker(ctx, c.seedBackupBucketQueue, "Seed Backup Bucket", c.seedBackupReconciler, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.seedQueue.ShutDown()
	c.seedBackupBucketQueue.ShutDown()
	c.seedLifecycleQueue.ShutDown()

	for {
		if c.seedQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running Seed worker and no items left in the queues. Terminated Seed controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d Seed worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.seedQueue.Len())
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}

// RunningWorkers returns the number of running workers.
func (c *Controller) RunningWorkers() int {
	return c.numberOfRunningWorkers
}

// CollectMetrics implements gardenmetrics.ControllerMetricsCollector interface
func (c *Controller) CollectMetrics(ch chan<- prometheus.Metric) {
	metric, err := prometheus.NewConstMetric(controllermanager.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "seed")
	if err != nil {
		controllermanager.ScrapeFailures.With(prometheus.Labels{"kind": "seed-controller"}).Inc()
		return
	}
	ch <- metric
}
