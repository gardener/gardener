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
	"fmt"
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Controller controls Seeds.
type Controller struct {
	k8sGardenClient        kubernetes.Interface
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory

	config *config.GardenletConfiguration

	control               ControlInterface
	heartbeatControl      HeartbeatControlInterface
	extensionCheckControl ExtensionCheckControlInterface

	recorder record.EventRecorder

	seedLister gardencorelisters.SeedLister
	seedSynced cache.InformerSynced

	controllerInstallationSynced cache.InformerSynced

	seedQueue               workqueue.RateLimitingInterface
	seedHeartbeatQueue      workqueue.RateLimitingInterface
	seedExtensionCheckQueue workqueue.RateLimitingInterface

	shootLister gardencorelisters.ShootLister

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewSeedController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <seedInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewSeedController(
	k8sGardenClient kubernetes.Interface,
	gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	secrets map[string]*corev1.Secret,
	imageVector imagevector.ImageVector,
	identity *gardencorev1beta1.Gardener,
	config *config.GardenletConfiguration,
	recorder record.EventRecorder,
) *Controller {
	var (
		gardenCoreV1beta1Informer = gardenCoreInformerFactory.Core().V1beta1()
		corev1Informer            = kubeInformerFactory.Core().V1()

		controllerInstallationInformer = gardenCoreV1beta1Informer.ControllerInstallations()
		seedInformer                   = gardenCoreV1beta1Informer.Seeds()

		controllerInstallationLister = controllerInstallationInformer.Lister()
		secretLister                 = corev1Informer.Secrets().Lister()
		seedLister                   = seedInformer.Lister()
		shootLister                  = gardenCoreV1beta1Informer.Shoots().Lister()
	)

	seedController := &Controller{
		k8sGardenClient:         k8sGardenClient,
		k8sGardenCoreInformers:  gardenCoreInformerFactory,
		control:                 NewDefaultControl(k8sGardenClient, gardenCoreInformerFactory, secrets, imageVector, identity, recorder, config, secretLister, shootLister),
		heartbeatControl:        NewDefaultHeartbeatControl(k8sGardenClient, gardenCoreV1beta1Informer, identity, config),
		extensionCheckControl:   NewDefaultExtensionCheckControl(k8sGardenClient.GardenCore(), controllerInstallationLister, metav1.Now),
		config:                  config,
		recorder:                recorder,
		seedLister:              seedLister,
		seedQueue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "seed"),
		seedHeartbeatQueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "seed-hearbeat"),
		seedExtensionCheckQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "seed-extension-check"),
		shootLister:             shootLister,
		workerCh:                make(chan int),
	}

	seedInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.SeedFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig), config.SeedSelector),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    seedController.seedAdd,
			UpdateFunc: seedController.seedUpdate,
			DeleteFunc: seedController.seedDelete,
		},
	})

	seedInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.SeedFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig), config.SeedSelector),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: seedController.seedHeartbeatAdd,
		},
	})
	seedController.seedSynced = seedInformer.Informer().HasSynced

	controllerInstallationInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.ControllerInstallationFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig), seedLister, config.SeedSelector),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    seedController.controllerInstallationOfSeedAdd,
			UpdateFunc: seedController.controllerInstallationOfSeedUpdate,
			DeleteFunc: seedController.controllerInstallationOfSeedDelete,
		},
	})
	seedController.controllerInstallationSynced = controllerInstallationInformer.Informer().HasSynced

	return seedController
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.seedSynced, c.controllerInstallationSynced) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for {
			select {
			case res := <-c.workerCh:
				c.numberOfRunningWorkers += res
				logger.Logger.Debugf("Current number of running Seed workers is %d", c.numberOfRunningWorkers)
			}
		}
	}()

	logger.Logger.Info("Seed controller initialized.")

	// Register Seed object if desired
	if c.config.SeedConfig != nil {
		seed := &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: c.config.SeedConfig.Name}}
		if _, err := controllerutil.CreateOrUpdate(ctx, c.k8sGardenClient.Client(), seed, func() error {
			seed.Labels = utils.MergeStringMaps(map[string]string{
				v1beta1constants.DeprecatedGardenRole: v1beta1constants.GardenRoleSeed,
				v1beta1constants.GardenRole:           v1beta1constants.GardenRoleSeed,
			}, c.config.SeedConfig.Labels)
			seed.Spec = c.config.SeedConfig.Seed.Spec
			return nil
		}); err != nil {
			panic(fmt.Errorf("could not register seed %q: %+v", seed.Name, err))
		}
	}

	for i := 0; i < workers; i++ {
		controllerutils.DeprecatedCreateWorker(ctx, c.seedQueue, "Seed", c.reconcileSeedKey, &waitGroup, c.workerCh)
		controllerutils.DeprecatedCreateWorker(ctx, c.seedHeartbeatQueue, "Seed Heartbeat", c.reconcileSeedHeartbeatKey, &waitGroup, c.workerCh)
		controllerutils.DeprecatedCreateWorker(ctx, c.seedExtensionCheckQueue, "Seed Extension Check", c.reconcileSeedExtensionCheckKey, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.seedQueue.ShutDown()
	c.seedHeartbeatQueue.ShutDown()
	c.seedExtensionCheckQueue.ShutDown()

	for {
		if c.seedQueue.Len() == 0 && c.seedHeartbeatQueue.Len() == 0 && c.seedExtensionCheckQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running Seed worker and no items left in the queues. Terminated Seed controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d Seed worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.seedQueue.Len()+c.seedHeartbeatQueue.Len()+c.seedExtensionCheckQueue.Len())
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
	metric, err := prometheus.NewConstMetric(gardenlet.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "seed")
	if err != nil {
		gardenlet.ScrapeFailures.With(prometheus.Labels{"kind": "seed-controller"}).Inc()
		return
	}
	ch <- metric
}
