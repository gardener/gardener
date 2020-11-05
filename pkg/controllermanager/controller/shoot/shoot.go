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

package shoot

import (
	"context"
	"fmt"
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllermanager"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	controllermanagerfeatures "github.com/gardener/gardener/pkg/controllermanager/features"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller controls Shoots.
type Controller struct {
	clientMap              clientmap.ClientMap
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory

	config                      *config.ControllerManagerConfiguration
	recorder                    record.EventRecorder
	maintenanceControl          MaintenanceControlInterface
	quotaControl                QuotaControlInterface
	hibernationScheduleRegistry HibernationScheduleRegistry

	shootLister     gardencorelisters.ShootLister
	configMapLister kubecorev1listers.ConfigMapLister

	shootMaintenanceQueue workqueue.RateLimitingInterface
	shootQuotaQueue       workqueue.RateLimitingInterface
	shootHibernationQueue workqueue.RateLimitingInterface
	shootReferenceQueue   workqueue.RateLimitingInterface
	configMapQueue        workqueue.RateLimitingInterface

	hasSyncedFuncs []cache.InformerSynced
	startFuncs     []func(<-chan struct{})

	shootRefReconciler reconcile.Reconciler

	numberOfRunningWorkers int
	workerCh               chan int
}

// NewShootController takes a ClientMap, a GardenerInformerFactory, a KubernetesInformerFactory, a
// ControllerManagerConfig struct and an EventRecorder to create a new Shoot controller.
func NewShootController(clientMap clientmap.ClientMap, k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory, kubeInformerFactory kubeinformers.SharedInformerFactory, config *config.ControllerManagerConfiguration, recorder record.EventRecorder) (*Controller, error) {
	var (
		gardenCoreV1beta1Informer = k8sGardenCoreInformers.Core().V1beta1()
		corev1Informer            = kubeInformerFactory.Core().V1()

		shootInformer = gardenCoreV1beta1Informer.Shoots()
		shootLister   = shootInformer.Lister()

		configMapInformer = corev1Informer.ConfigMaps()
		configMapLister   = configMapInformer.Lister()
	)

	shootController := &Controller{
		clientMap:              clientMap,
		k8sGardenCoreInformers: k8sGardenCoreInformers,

		config:                      config,
		recorder:                    recorder,
		maintenanceControl:          NewDefaultMaintenanceControl(clientMap, gardenCoreV1beta1Informer, config.Controllers.ShootMaintenance, recorder),
		quotaControl:                NewDefaultQuotaControl(clientMap, gardenCoreV1beta1Informer),
		hibernationScheduleRegistry: NewHibernationScheduleRegistry(),

		shootLister:     shootLister,
		configMapLister: configMapLister,

		shootMaintenanceQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-maintenance"),
		shootQuotaQueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-quota"),
		shootHibernationQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-hibernation"),
		shootReferenceQueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shoot-references"),
		configMapQueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "configmaps"),

		workerCh: make(chan int),
	}

	shootInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.shootMaintenanceAdd,
		UpdateFunc: shootController.shootMaintenanceUpdate,
		DeleteFunc: shootController.shootMaintenanceDelete,
	})

	shootInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.shootQuotaAdd,
		DeleteFunc: shootController.shootQuotaDelete,
	})

	shootInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.shootHibernationAdd,
		UpdateFunc: shootController.shootHibernationUpdate,
		DeleteFunc: shootController.shootHibernationDelete,
	})

	configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.configMapAdd,
		UpdateFunc: shootController.configMapUpdate,
	})

	gardenClient, err := clientMap.GetClient(context.TODO(), keys.ForGarden())
	if err != nil {
		return nil, err
	}

	runtimeShootInformer, err := gardenClient.Cache().GetInformer(context.TODO(), &gardencorev1beta1.Shoot{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Shoot Informer: %w", err)
	}
	runtimeShootInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    shootController.shootReferenceAdd,
		UpdateFunc: shootController.shootReferenceUpdate,
	})

	shootController.hasSyncedFuncs = []cache.InformerSynced{
		shootInformer.Informer().HasSynced,
		runtimeShootInformer.HasSynced,
		gardenCoreV1beta1Informer.Quotas().Informer().HasSynced,
		configMapInformer.Informer().HasSynced,
	}

	runtimeSecretLister := func(ctx context.Context, secretList *corev1.SecretList, opts ...client.ListOption) error {
		return gardenClient.Cache().List(ctx, secretList, opts...)
	}
	runtimeConfigMapLister := func(ctx context.Context, configMapList *corev1.ConfigMapList, opts ...client.ListOption) error {
		return gardenClient.Cache().List(ctx, configMapList, opts...)
	}

	// If cache is not enabled, set up a dedicated informer which only considers objects which are not gardener managed.
	// Large gardener environments hold many secrets and with a proper cache we can compensate the load the controller puts on the API server.
	if !controllermanagerfeatures.FeatureGate.Enabled(features.CachedRuntimeClients) {
		factory := kubeinformers.NewSharedInformerFactoryWithOptions(gardenClient.Kubernetes(), 0,
			kubeinformers.WithTweakListOptions(func(opts *metav1.ListOptions) {
				opts.LabelSelector = UserManagedSelector.String()
			}))
		secretInformer := factory.Core().V1().Secrets()

		runtimeSecretLister = func(ctx context.Context, secretList *corev1.SecretList, opts ...client.ListOption) error {
			listOpts := &client.ListOptions{}
			for _, opt := range opts {
				opt.ApplyToList(listOpts)
			}

			secrets, err := secretInformer.Lister().Secrets(listOpts.Namespace).List(listOpts.LabelSelector)
			if err != nil {
				return err
			}
			for _, secret := range secrets {
				secretList.Items = append(secretList.Items, *secret)
			}

			return nil
		}

		shootController.hasSyncedFuncs = append(shootController.hasSyncedFuncs, secretInformer.Informer().HasSynced)
		shootController.startFuncs = append(shootController.startFuncs, factory.Start)
	}

	shootController.shootRefReconciler = NewShootReferenceReconciler(logger.Logger, clientMap, runtimeSecretLister, runtimeConfigMapLister, config.Controllers.ShootReference)

	return shootController, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, shootMaintenanceWorkers, shootQuotaWorkers, shootHibernationWorkers, shootReferenceWorkers int) {
	for _, start := range c.startFuncs {
		start(ctx.Done())
	}

	var waitGroup sync.WaitGroup
	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running Shoot workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("Shoot controller initialized.")

	for i := 0; i < shootMaintenanceWorkers; i++ {
		controllerutils.DeprecatedCreateWorker(ctx, c.shootMaintenanceQueue, "Shoot Maintenance", c.reconcileShootMaintenanceKey, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootQuotaWorkers; i++ {
		controllerutils.DeprecatedCreateWorker(ctx, c.shootQuotaQueue, "Shoot Quota", c.reconcileShootQuotaKey, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootHibernationWorkers; i++ {
		controllerutils.DeprecatedCreateWorker(ctx, c.shootHibernationQueue, "Scheduled Shoot Hibernation", c.reconcileShootHibernationKey, &waitGroup, c.workerCh)
	}
	for i := 0; i < shootMaintenanceWorkers; i++ {
		controllerutils.DeprecatedCreateWorker(ctx, c.configMapQueue, "ConfigMap", c.reconcileConfigMapKey, &waitGroup, c.workerCh)
	}

	for i := 0; i < shootReferenceWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.shootReferenceQueue, "ShootReference", c.shootRefReconciler, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.shootMaintenanceQueue.ShutDown()
	c.shootQuotaQueue.ShutDown()
	c.shootHibernationQueue.ShutDown()
	c.configMapQueue.ShutDown()
	c.shootReferenceQueue.ShutDown()

	for {
		var (
			shootMaintenanceQueueLength = c.shootMaintenanceQueue.Len()
			shootQuotaQueueLength       = c.shootQuotaQueue.Len()
			shootHibernationQueueLength = c.shootHibernationQueue.Len()
			configMapQueueLength        = c.configMapQueue.Len()
			referenceQueueLength        = c.shootReferenceQueue.Len()
			queueLengths                = shootMaintenanceQueueLength + shootQuotaQueueLength + shootHibernationQueueLength + configMapQueueLength + referenceQueueLength
		)
		if queueLengths == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running Shoot worker and no items left in the queues. Terminated Shoot controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d Shoot worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, queueLengths)
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
	metric, err := prometheus.NewConstMetric(controllermanager.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "shoot")
	if err != nil {
		controllermanager.ScrapeFailures.With(prometheus.Labels{"kind": "shoot-controller"}).Inc()
		return
	}
	ch <- metric
}
