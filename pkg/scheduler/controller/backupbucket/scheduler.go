// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/scheduler"
	"github.com/gardener/gardener/pkg/scheduler/apis/config"
)

// SchedulerController controls Seeds.
type SchedulerController struct {
	config *config.SchedulerConfiguration

	reconciler reconcile.Reconciler
	recorder   record.EventRecorder

	backupBucketQueue  workqueue.RateLimitingInterface
	backupBucketSynced cache.InformerSynced

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewGardenerScheduler takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a <sharedInformerFactory>, a struct containing the scheduler configuration and a <recorder> for
// event recording. It creates a new NewGardenerScheduler.
func NewGardenerScheduler(ctx context.Context, k8sGardenClient kubernetes.Interface, k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory, config *config.SchedulerConfiguration, recorder record.EventRecorder) *SchedulerController {
	var (
		gardencorev1beta1Informer = k8sGardenCoreInformers.Core().V1beta1()
		backupBucketInformer      = gardencorev1beta1Informer.BackupBuckets()
		backupBuckerQueue         = workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(config.Schedulers.BackupBucket.RetrySyncPeriod.Duration, 12*time.Hour), "gardener-backup-bucket-scheduler")
	)

	schedulerController := &SchedulerController{
		reconciler:        newReconciler(ctx, k8sGardenClient.Client(), recorder),
		config:            config,
		recorder:          recorder,
		backupBucketQueue: backupBuckerQueue,
		workerCh:          make(chan int),
	}

	backupBucketInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    schedulerController.backupBucketAdd,
		UpdateFunc: schedulerController.backupBucketUpdate,
	})

	schedulerController.backupBucketSynced = backupBucketInformer.Informer().HasSynced

	return schedulerController
}

// Run runs the SchedulerController until the given stop channel can be read from.
func (c *SchedulerController) Run(ctx context.Context, k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory) {
	var waitGroup sync.WaitGroup

	k8sGardenCoreInformers.Start(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), c.backupBucketSynced) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running Scheduler workers is %d", c.numberOfRunningWorkers)
		}
	}()

	for i := 0; i < c.config.Schedulers.Shoot.ConcurrentSyncs; i++ {
		controllerutils.CreateWorker(ctx, c.backupBucketQueue, "gardener--backup-bucket-scheduler", c.reconciler, &waitGroup, c.workerCh)
	}

	logger.Logger.Infof("BackupBucket Scheduler controller initialized with %d workers", c.config.Schedulers.Shoot.ConcurrentSyncs)

	// Shutdown handling
	<-ctx.Done()
	c.backupBucketQueue.ShutDown()

	for {
		if c.backupBucketQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running Scheduler worker and no items left in the queues. Terminated Scheduler controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d Scheduler worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.backupBucketQueue.Len())
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}

// RunningWorkers returns the number of running workers.
func (c *SchedulerController) RunningWorkers() int {
	return c.numberOfRunningWorkers
}

// CollectMetrics implements gardenmetrics.ControllerMetricsCollector interface
func (c *SchedulerController) CollectMetrics(ch chan<- prometheus.Metric) {
	metric, err := prometheus.NewConstMetric(scheduler.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "seed")
	if err != nil {
		scheduler.ScrapeFailures.With(prometheus.Labels{"kind": "gardener-backup-bucket-scheduler"}).Inc()
		return
	}
	ch <- metric
}
