// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
)

// Controller controls Seeds.
type Controller struct {
	clientMap              clientmap.ClientMap
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory

	config   *config.ControllerManagerConfiguration
	control  ControlInterface
	recorder record.EventRecorder

	seedLister gardencorelisters.SeedLister
	seedQueue  workqueue.RateLimitingInterface
	seedSynced cache.InformerSynced

	shootLister gardencorelisters.ShootLister
	shootSynced cache.InformerSynced

	leaseSynced cache.InformerSynced

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewSeedController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <gardenInformerFactory>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewSeedController(clientMap clientmap.ClientMap, gardenInformerFactory gardencoreinformers.SharedInformerFactory, kubeInformerFactory kubeinformers.SharedInformerFactory,
	config *config.ControllerManagerConfiguration, recorder record.EventRecorder) *Controller {
	var (
		gardenCoreV1beta1Informer = gardenInformerFactory.Core().V1beta1()

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
		control:                NewDefaultControl(clientMap, leaseLister, shootLister, config),
		recorder:               recorder,
		seedLister:             seedLister,
		shootLister:            shootLister,
		seedQueue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Seed"),
		workerCh:               make(chan int),
	}

	seedInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: seedController.seedAdd,
	})
	seedController.seedSynced = seedInformer.Informer().HasSynced
	seedController.shootSynced = shootInformer.Informer().HasSynced
	seedController.leaseSynced = leaseInformer.Informer().HasSynced

	return seedController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.seedSynced, c.shootSynced, c.leaseSynced) {
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
		controllerutils.DeprecatedCreateWorker(ctx, c.seedQueue, "Seed", c.reconcileSeedKey, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.seedQueue.ShutDown()

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
