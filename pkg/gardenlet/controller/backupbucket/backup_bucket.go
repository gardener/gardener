// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"
	"fmt"
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// finalizerName is the backupbucket controller finalizer.
	finalizerName = "core.gardener.cloud/backupbucket"
)

// Controller controls BackupBuckets.
type Controller struct {
	gardenClient client.Client
	config       *config.GardenletConfiguration
	reconciler   reconcile.Reconciler
	recorder     record.EventRecorder

	backupBucketInformer runtimecache.Informer
	backupBucketQueue    workqueue.RateLimitingInterface

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewBackupBucketController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <backupBucketInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewBackupBucketController(ctx context.Context, clientMap clientmap.ClientMap, config *config.GardenletConfiguration, recorder record.EventRecorder) (*Controller, error) {
	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, fmt.Errorf("failed to get garden client: %w", err)
	}

	backupBucketInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.BackupBucket{})
	if err != nil {
		return nil, fmt.Errorf("failed to get BackupBucket Informer: %w", err)
	}

	return &Controller{
		gardenClient:         gardenClient.Client(),
		config:               config,
		reconciler:           newReconciler(ctx, clientMap, recorder, config),
		recorder:             recorder,
		backupBucketInformer: backupBucketInformer,
		backupBucketQueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "BackupBucket"),
		workerCh:             make(chan int),
	}, nil
}

// Run runs the Controller until the given stop channel is closed.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	c.backupBucketInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.BackupBucketFilterFunc(ctx, c.gardenClient, confighelper.SeedNameFromSeedConfig(c.config.SeedConfig), c.config.SeedSelector),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    c.backupBucketAdd,
			UpdateFunc: c.backupBucketUpdate,
			DeleteFunc: c.backupBucketDelete,
		},
	})

	if !cache.WaitForCacheSync(ctx.Done(), c.backupBucketInformer.HasSynced) {
		logger.Logger.Fatal("Timed out waiting for BackupBucket Informer to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running BackupBucket workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("BackupBucket controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.backupBucketQueue, "backupbucket", c.reconciler, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.backupBucketQueue.ShutDown()

	for {
		if c.backupBucketQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Info("No running BackupBucket worker and no items left in the queues. Terminated BackupBucket controller...")
			break
		}
		logger.Logger.Infof("Waiting for %d BackupBucket worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.backupBucketQueue.Len())
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
	metric, err := prometheus.NewConstMetric(gardenlet.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "backupbucket")
	if err != nil {
		gardenlet.ScrapeFailures.With(prometheus.Labels{"kind": "backupbucket-controller"}).Inc()
		return
	}
	ch <- metric
}
