// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprofile

import (
	"context"
	"sync"
	"time"

	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/controllermanager"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// Controller controls CloudProfiles.
type Controller struct {
	clientMap              clientmap.ClientMap
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory

	control ControlInterface

	cloudProfileLister gardencorelisters.CloudProfileLister
	cloudProfileQueue  workqueue.RateLimitingInterface
	cloudprofileSynced cache.InformerSynced

	shootLister gardencorelisters.ShootLister

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewCloudProfileController takes a Kubernetes client <k8sGardenClient> and a <k8sGardenCoreInformers> for the Garden clusters.
// It creates and return a new Garden controller to control CloudProfiles.
func NewCloudProfileController(clientMap clientmap.ClientMap, k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory, recorder record.EventRecorder) *Controller {
	var (
		gardenCoreV1beta1Informer = k8sGardenCoreInformers.Core().V1beta1()
		cloudProfileInformer      = gardenCoreV1beta1Informer.CloudProfiles()
		shootLister               = gardenCoreV1beta1Informer.Shoots().Lister()
	)

	cloudProfileController := &Controller{
		clientMap:              clientMap,
		k8sGardenCoreInformers: k8sGardenCoreInformers,
		cloudProfileLister:     cloudProfileInformer.Lister(),
		cloudProfileQueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "cloudprofile"),
		shootLister:            shootLister,
		control:                NewDefaultControl(clientMap, shootLister, recorder),
		workerCh:               make(chan int),
	}

	cloudProfileInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    cloudProfileController.cloudProfileAdd,
		UpdateFunc: cloudProfileController.cloudProfileUpdate,
		DeleteFunc: cloudProfileController.cloudProfileDelete,
	})
	cloudProfileController.cloudprofileSynced = cloudProfileInformer.Informer().HasSynced

	return cloudProfileController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	// Check if informers cache has been populated
	if !cache.WaitForCacheSync(ctx.Done(), c.cloudprofileSynced) {
		logger.Logger.Error("Time out waiting for caches to sync")
		return
	}

	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running CloudProfile workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("CloudProfile controller initialized.")

	// Start the workers
	for i := 0; i < workers; i++ {
		controllerutils.DeprecatedCreateWorker(ctx, c.cloudProfileQueue, "cloudprofile", c.reconcileCloudProfileKey, &waitGroup, c.workerCh)
	}

	<-ctx.Done()
	c.cloudProfileQueue.ShutDown()

	for {
		if c.cloudProfileQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running CloudProfile worker and no items left in the queues. Terminated CloudProfile controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d CloudProfile worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.cloudProfileQueue.Len())
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
	metric, err := prometheus.NewConstMetric(controllermanager.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "cloudprofile")
	if err != nil {
		controllermanager.ScrapeFailures.With(prometheus.Labels{"kind": "cloudprofile-controller"}).Inc()
		return
	}
	ch <- metric
}
