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

package shoot

import (
	"context"
	"sync"
	"time"

	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/scheduler"
	"github.com/gardener/gardener/pkg/scheduler/apis/config"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// SchedulerController controls Seeds.
type SchedulerController struct {
	k8sGardenClient        kubernetes.Interface
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory

	config *config.SchedulerConfiguration

	control  SchedulerInterface
	recorder record.EventRecorder

	cloudProfileLister gardencorelisters.CloudProfileLister
	cloudProfileSynced cache.InformerSynced

	seedLister gardencorelisters.SeedLister
	seedSynced cache.InformerSynced

	shootLister gardencorelisters.ShootLister
	shootSynced cache.InformerSynced
	shootQueue  workqueue.RateLimitingInterface

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewGardenerScheduler takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a <sharedInformerFactory>, a struct containing the scheduler configuration and a <recorder> for
// event recording. It creates a new NewGardenerScheduler.
func NewGardenerScheduler(k8sGardenClient kubernetes.Interface, gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory, config *config.SchedulerConfiguration, recorder record.EventRecorder) *SchedulerController {
	var (
		coreV1beta1Informer = gardenCoreInformerFactory.Core().V1beta1()

		shootInformer        = coreV1beta1Informer.Shoots()
		shootLister          = shootInformer.Lister()
		seedInformer         = coreV1beta1Informer.Seeds()
		seedLister           = seedInformer.Lister()
		cloudProfileInformer = coreV1beta1Informer.CloudProfiles()
		cloudProfileLister   = cloudProfileInformer.Lister()
		shootQueue           = workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(config.Schedulers.Shoot.RetrySyncPeriod.Duration, 12*time.Hour), "gardener-shoot-scheduler")
	)

	schedulerController := &SchedulerController{
		k8sGardenClient:        k8sGardenClient,
		k8sGardenCoreInformers: gardenCoreInformerFactory,
		control:                NewDefaultControl(k8sGardenClient, gardenCoreInformerFactory, recorder, config, shootLister, seedLister, cloudProfileLister),
		config:                 config,
		recorder:               recorder,
		cloudProfileLister:     cloudProfileLister,
		seedLister:             seedLister,
		shootQueue:             shootQueue,
		shootLister:            shootLister,
		workerCh:               make(chan int),
	}

	shootInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    schedulerController.shootAdd,
		UpdateFunc: schedulerController.shootUpdate,
	})
	schedulerController.cloudProfileSynced = cloudProfileInformer.Informer().HasSynced
	schedulerController.seedSynced = seedInformer.Informer().HasSynced
	schedulerController.shootSynced = shootInformer.Informer().HasSynced

	return schedulerController
}

// Run runs the SchedulerController until the given stop channel can be read from.
func (c *SchedulerController) Run(ctx context.Context, k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory) {
	var waitGroup sync.WaitGroup

	k8sGardenCoreInformers.Start(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), c.cloudProfileSynced, c.seedSynced, c.shootSynced) {
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
		controllerutils.DeprecatedCreateWorker(ctx, c.shootQueue, "gardener-scheduler", func(key string) error { return c.reconcileShootKey(ctx, key) }, &waitGroup, c.workerCh)
	}

	logger.Logger.Infof("Shoot Scheduler controller initialized with %d workers  (with Strategy: %s)", c.config.Schedulers.Shoot.ConcurrentSyncs, c.config.Schedulers.Shoot.Strategy)

	<-ctx.Done()
	c.shootQueue.ShutDown()

	for {
		if c.shootQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running Scheduler worker and no items left in the queues. Terminated Scheduler controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d Scheduler worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.shootQueue.Len())
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
		scheduler.ScrapeFailures.With(prometheus.Labels{"kind": "gardener-shoot-scheduler"}).Inc()
		return
	}
	ch <- metric
}
