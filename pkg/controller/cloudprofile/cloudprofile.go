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

package cloudprofile

import (
	"context"
	"sync"
	"time"

	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllerutils "github.com/gardener/gardener/pkg/controller/utils"
	"github.com/gardener/gardener/pkg/logger"

	gardenmetrics "github.com/gardener/gardener/pkg/metrics"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// Controller controls CloudProfiles.
type Controller struct {
	k8sGardenClient    kubernetes.Client
	k8sGardenInformers gardeninformers.SharedInformerFactory

	control ControlInterface

	cloudProfileLister gardenlisters.CloudProfileLister
	cloudProfileQueue  workqueue.RateLimitingInterface
	cloudprofileSynced cache.InformerSynced

	seedLister  gardenlisters.SeedLister
	shootLister gardenlisters.ShootLister

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewCloudProfileController takes a Kubernetes client <k8sGardenClient> and a <k8sGardenInformers> for the Garden clusters.
// It creates and return a new Garden controller to control CloudProfiles.
func NewCloudProfileController(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.SharedInformerFactory) *Controller {
	var (
		gardenv1beta1Informer = k8sGardenInformers.Garden().V1beta1()
		cloudProfileInformer  = gardenv1beta1Informer.CloudProfiles()
		seedLister            = gardenv1beta1Informer.Seeds().Lister()
		shootLister           = gardenv1beta1Informer.Shoots().Lister()
	)

	cloudProfileController := &Controller{
		k8sGardenClient:    k8sGardenClient,
		k8sGardenInformers: k8sGardenInformers,
		cloudProfileLister: cloudProfileInformer.Lister(),
		cloudProfileQueue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "cloudprofile"),
		seedLister:         seedLister,
		shootLister:        shootLister,
		control:            NewDefaultControl(k8sGardenClient, seedLister, shootLister),
		workerCh:           make(chan int),
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
		for {
			select {
			case res := <-c.workerCh:
				c.numberOfRunningWorkers += res
				logger.Logger.Debugf("Current number of running CloudProfile workers is %d", c.numberOfRunningWorkers)
			}
		}
	}()

	logger.Logger.Info("CloudProfile controller initialized.")

	// Start the workers
	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.cloudProfileQueue, "cloudprofile", c.reconcileCloudProfileKey, &waitGroup, c.workerCh)
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
	metric, err := prometheus.NewConstMetric(gardenmetrics.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "cloudprofile")
	if err != nil {
		gardenmetrics.ScrapeFailures.With(prometheus.Labels{"kind": "cloudprofile-controller"}).Inc()
		return
	}
	ch <- metric
}
