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

package backupinfrastructure

import (
	"context"
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/apis/componentconfig"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllerutils "github.com/gardener/gardener/pkg/controller/utils"
	"github.com/gardener/gardener/pkg/logger"
	gardenmetrics "github.com/gardener/gardener/pkg/metrics"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// Controller controls BackupInfrastructures.
type Controller struct {
	k8sGardenClient    kubernetes.Client
	k8sGardenInformers gardeninformers.SharedInformerFactory

	config      *componentconfig.ControllerManagerConfiguration
	control     ControlInterface
	recorder    record.EventRecorder
	secrets     map[string]*corev1.Secret
	imageVector imagevector.ImageVector

	backupInfrastructureLister gardenlisters.BackupInfrastructureLister
	backupInfrastructureQueue  workqueue.RateLimitingInterface
	backupInfrastructureSynced cache.InformerSynced

	seedSynced             cache.InformerSynced
	workerCh               chan int
	numberOfRunningWorkers int
}

// NewBackupInfrastructureController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <backupInfrastructureInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewBackupInfrastructureController(k8sGardenClient kubernetes.Client, gardenInformerFactory gardeninformers.SharedInformerFactory, config *componentconfig.ControllerManagerConfiguration, identity *gardenv1beta1.Gardener, gardenNamespace string, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector, recorder record.EventRecorder) *Controller {
	var (
		gardenv1beta1Informer        = gardenInformerFactory.Garden().V1beta1()
		backupInfrastructureInformer = gardenv1beta1Informer.BackupInfrastructures()
		backupInfrastructureLister   = backupInfrastructureInformer.Lister()
		backupInfrastructureUpdater  = NewRealUpdater(k8sGardenClient, backupInfrastructureLister)
	)

	backupInfrastructureController := &Controller{
		k8sGardenClient:            k8sGardenClient,
		k8sGardenInformers:         gardenInformerFactory,
		config:                     config,
		control:                    NewDefaultControl(k8sGardenClient, gardenv1beta1Informer, secrets, imageVector, identity, config, recorder, backupInfrastructureUpdater),
		recorder:                   recorder,
		secrets:                    secrets,
		imageVector:                imageVector,
		backupInfrastructureLister: backupInfrastructureLister,
		backupInfrastructureQueue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "BackupInfrastructure"),
		workerCh:                   make(chan int),
	}

	backupInfrastructureInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    backupInfrastructureController.backupInfrastructureAdd,
		UpdateFunc: backupInfrastructureController.backupInfrastructureUpdate,
		DeleteFunc: backupInfrastructureController.backupInfrastructureDelete,
	})
	backupInfrastructureController.backupInfrastructureSynced = backupInfrastructureInformer.Informer().HasSynced
	backupInfrastructureController.seedSynced = gardenv1beta1Informer.Seeds().Informer().HasSynced
	return backupInfrastructureController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.backupInfrastructureSynced, c.seedSynced) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for {
			select {
			case res := <-c.workerCh:
				c.numberOfRunningWorkers += res
				logger.Logger.Debugf("Current number of running BackupInfrastructure workers is %d", c.numberOfRunningWorkers)
			}
		}
	}()

	logger.Logger.Info("BackupInfrastructure controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.backupInfrastructureQueue, "backupinfrastructure", c.reconcileBackupInfrastructureKey, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.backupInfrastructureQueue.ShutDown()

	for {
		if c.backupInfrastructureQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Info("No running BackupInfrastructure worker and no items left in the queues. Terminated BackupInfrastructure controller...")
			break
		}
		logger.Logger.Infof("Waiting for %d BackupInfrastructure worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.backupInfrastructureQueue.Len())
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
	metric, err := prometheus.NewConstMetric(gardenmetrics.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "backupinfrastructure")
	if err != nil {
		gardenmetrics.ScrapeFailures.With(prometheus.Labels{"kind": "backupinfrastructure-controller"}).Inc()
		return
	}
	ch <- metric
}
