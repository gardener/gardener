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

package secretbinding

import (
	"context"
	"sync"
	"time"

	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllerutils "github.com/gardener/gardener/pkg/controller/utils"
	"github.com/gardener/gardener/pkg/logger"
	gardenmetrics "github.com/gardener/gardener/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// Controller controls SecretBindings.
type Controller struct {
	k8sGardenClient    kubernetes.Client
	k8sGardenInformers gardeninformers.SharedInformerFactory

	k8sInformers kubeinformers.SharedInformerFactory

	control  ControlInterface
	recorder record.EventRecorder

	secretBindingLister gardenlisters.SecretBindingLister
	secretBindingQueue  workqueue.RateLimitingInterface
	secretBindingSynced cache.InformerSynced

	shootLister gardenlisters.ShootLister

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewSecretBindingController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <secretBindingInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewSecretBindingController(k8sGardenClient kubernetes.Client, gardenInformerFactory gardeninformers.SharedInformerFactory, kubeInformerFactory kubeinformers.SharedInformerFactory, recorder record.EventRecorder) *Controller {
	var (
		gardenv1beta1Informer = gardenInformerFactory.Garden().V1beta1()
		corev1Informer        = kubeInformerFactory.Core().V1()

		secretBindingInformer = gardenv1beta1Informer.SecretBindings()
		secretBindingLister   = secretBindingInformer.Lister()
		secretLister          = corev1Informer.Secrets().Lister()
		shootLister           = gardenv1beta1Informer.Shoots().Lister()
	)

	secretBindingController := &Controller{
		k8sGardenClient:     k8sGardenClient,
		k8sGardenInformers:  gardenInformerFactory,
		control:             NewDefaultControl(k8sGardenClient, gardenInformerFactory, recorder, secretLister, shootLister),
		recorder:            recorder,
		secretBindingLister: secretBindingLister,
		secretBindingQueue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "SecretBinding"),
		shootLister:         shootLister,
		workerCh:            make(chan int),
	}

	secretBindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    secretBindingController.secretBindingAdd,
		UpdateFunc: secretBindingController.secretBindingUpdate,
		DeleteFunc: secretBindingController.secretBindingDelete,
	})
	secretBindingController.secretBindingSynced = secretBindingInformer.Informer().HasSynced

	return secretBindingController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.secretBindingSynced) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for {
			select {
			case res := <-c.workerCh:
				c.numberOfRunningWorkers += res
				logger.Logger.Debugf("Current number of running SecretBinding workers is %d", c.numberOfRunningWorkers)
			}
		}
	}()

	logger.Logger.Info("SecretBinding controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.secretBindingQueue, "SecretBinding", c.reconcileSecretBindingKey, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.secretBindingQueue.ShutDown()

	for {
		if c.secretBindingQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running SecretBinding worker and no items left in the queues. Terminated SecretBinding controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d SecretBinding worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.secretBindingQueue.Len())
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
	metric, err := prometheus.NewConstMetric(gardenmetrics.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "secretbinding")
	if err != nil {
		gardenmetrics.ScrapeFailures.With(prometheus.Labels{"kind": "secretbinding-controller"}).Inc()
		return
	}
	ch <- metric
}
