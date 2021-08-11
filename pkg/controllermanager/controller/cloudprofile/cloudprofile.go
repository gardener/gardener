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
	"fmt"
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllermanager"
	"github.com/gardener/gardener/pkg/controllerutils"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller controls CloudProfiles.
type Controller struct {
	reconciler     reconcile.Reconciler
	hasSyncedFuncs []cache.InformerSynced

	cloudProfileQueue      workqueue.RateLimitingInterface
	workerCh               chan int
	numberOfRunningWorkers int

	logger *logrus.Logger
}

// NewCloudProfileController takes a Kubernetes client <k8sGardenClient> and a <k8sGardenCoreInformers> for the Garden clusters.
// It creates and return a new Garden controller to control CloudProfiles.
func NewCloudProfileController(
	ctx context.Context,
	clientMap clientmap.ClientMap,
	recorder record.EventRecorder,
	logger *logrus.Logger,
) (
	*Controller,
	error,
) {
	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, err
	}

	cloudProfileInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.CloudProfile{})
	if err != nil {
		return nil, fmt.Errorf("failed to get CloudProfile Informer: %w", err)
	}

	cloudProfileController := &Controller{
		cloudProfileQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "cloudprofile"),
		reconciler:        NewCloudProfileReconciler(logger, gardenClient.Client(), recorder),
		workerCh:          make(chan int),
		logger:            logger,
	}

	cloudProfileInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    cloudProfileController.cloudProfileAdd,
		UpdateFunc: cloudProfileController.cloudProfileUpdate,
		DeleteFunc: cloudProfileController.cloudProfileDelete,
	})

	cloudProfileController.hasSyncedFuncs = append(cloudProfileController.hasSyncedFuncs, cloudProfileInformer.HasSynced)

	return cloudProfileController, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	// Check if informers cache has been populated
	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		c.logger.Error("Time out waiting for caches to sync")
		return
	}

	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			c.logger.Debugf("Current number of running CloudProfile workers is %d", c.numberOfRunningWorkers)
		}
	}()

	c.logger.Info("CloudProfile controller initialized.")

	// Start the workers
	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.cloudProfileQueue, "CloudProfile", c.reconciler, &waitGroup, c.workerCh)
	}

	<-ctx.Done()
	c.cloudProfileQueue.ShutDown()

	for {
		if c.cloudProfileQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			c.logger.Debug("No running CloudProfile worker and no items left in the queues. Terminated CloudProfile controller...")
			break
		}
		c.logger.Debugf("Waiting for %d CloudProfile worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.cloudProfileQueue.Len())
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
