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

package federatedseed

import (
	"context"
	"sync"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	extensionsclientset "github.com/gardener/gardener/pkg/client/extensions/clientset/versioned"
	extensionsinformers "github.com/gardener/gardener/pkg/client/extensions/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/gardenlet/controller/federatedseed/extensions"
	"github.com/gardener/gardener/pkg/logger"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"

	dnsclientset "github.com/gardener/external-dns-management/pkg/client/dns/clientset/versioned"
	dnsinformers "github.com/gardener/external-dns-management/pkg/client/dns/informers/externalversions"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller is responsible for maintaining multiple federated Seeds' controllers
type Controller struct {
	federatedSeedControllerManager *federatedSeedControllerManager
	k8sGardenClient                kubernetes.Interface

	config *config.GardenletConfiguration

	seedLister    gardencorelisters.SeedLister
	seedHasSynced cache.InformerSynced
	seedQueue     workqueue.RateLimitingInterface

	recorder record.EventRecorder

	numberOfRunningWorkers int
	workerCh               chan int
}

// NewFederatedSeedController creates new controller that reconciles extension resources.
func NewFederatedSeedController(k8sGardenClient kubernetes.Interface, gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory, config *config.GardenletConfiguration, recorder record.EventRecorder) *Controller {
	var seedInformer = gardenCoreInformerFactory.Core().V1beta1().Seeds()

	controller := &Controller{
		k8sGardenClient: k8sGardenClient,
		seedLister:      seedInformer.Lister(),
		seedQueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "seed-controllers"),
		config:          config,
		workerCh:        make(chan int),
		recorder:        recorder,
	}

	controller.federatedSeedControllerManager = &federatedSeedControllerManager{
		controllers: make(map[string]*extensionsController),
	}

	seedInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.SeedFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig), config.SeedSelector),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.seedAdd,
			UpdateFunc: controller.seedUpdate,
			DeleteFunc: controller.seedDelete,
		},
	})

	controller.seedHasSynced = seedInformer.Informer().HasSynced
	return controller
}

func (c *Controller) seedAdd(obj interface{}) {
	seedObj := obj.(*gardencorev1beta1.Seed)
	bootstrappedCondition := helper.GetCondition(seedObj.Status.Conditions, gardencorev1beta1.SeedBootstrapped)
	if bootstrappedCondition == nil || bootstrappedCondition.Status != gardencorev1beta1.ConditionTrue {
		return
	}
	c.addSeedToQueue(obj)
}

func (c *Controller) addSeedToQueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.seedQueue.Add(key)
}

func (c *Controller) seedUpdate(_, newObj interface{}) {
	var newSeedObj = newObj.(*gardencorev1beta1.Seed)

	newSeedBootstrappedCondition := helper.GetCondition(newSeedObj.Status.Conditions, gardencorev1beta1.SeedBootstrapped)
	_, found := c.federatedSeedControllerManager.controllers[newSeedObj.Name]
	if !found && newSeedBootstrappedCondition != nil && newSeedBootstrappedCondition.Status == gardencorev1beta1.ConditionTrue {
		c.addSeedToQueue(newSeedObj)
	}
}

func (c *Controller) seedDelete(obj interface{}) {
	seed := obj.(*gardencorev1beta1.Seed)
	c.federatedSeedControllerManager.removeController(seed.GetName())
}

// Run starts the FederatedSeed Controller
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.seedHasSynced) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running Seed Federated controller workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Infof("Starting federated controllers")
	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.seedQueue, "Seed", c.createReconcileSeedRequestFunc(ctx), &waitGroup, c.workerCh)
	}

	<-ctx.Done()

	logger.Logger.Infof("Shutting down federated controllers")
	c.federatedSeedControllerManager.ShutDownControllers()
	c.seedQueue.ShutDown()
	waitGroup.Wait()
}

func (c *Controller) createReconcileSeedRequestFunc(ctx context.Context) reconcile.Func {
	return func(req reconcile.Request) (reconcile.Result, error) {
		seed, err := c.seedLister.Get(req.Name)
		if apierrors.IsNotFound(err) {
			logger.Logger.Infof("Skipping - Seed %s was not found", req.Name)
			return reconcile.Result{}, nil
		}
		if err != nil {
			return reconcile.Result{}, err
		}

		if seed.DeletionTimestamp != nil {
			c.federatedSeedControllerManager.removeController(seed.GetName())
			return reconcile.Result{}, nil
		}

		condition := helper.GetCondition(seed.Status.Conditions, gardencorev1beta1.SeedBootstrapped)
		if condition.Status != gardencorev1beta1.ConditionTrue {
			return reconcile.Result{}, nil
		}

		if err := c.federatedSeedControllerManager.createControllers(ctx, seed.GetName(), c.k8sGardenClient, c.config, c.recorder); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}
}

type federatedSeedControllerManager struct {
	controllers map[string]*extensionsController
	lock        sync.RWMutex
}

type extensionsController struct {
	controller *extensions.Controller
	cancelFunc context.CancelFunc
}

func (s *extensionsController) Stop() {
	s.cancelFunc()
	s.controller.Stop()
}

func (f *federatedSeedControllerManager) createControllers(ctx context.Context, seedName string, k8sGardenClient kubernetes.Interface, config *config.GardenletConfiguration, recorder record.EventRecorder) error {
	if _, found := f.controllers[seedName]; found {
		logger.Logger.Infof("Controllers are already started for seed %s", seedName)
		return nil
	}

	k8sSeedClient, err := seedpkg.GetSeedClient(ctx, k8sGardenClient.Client(), config.SeedClientConnection.ClientConnectionConfiguration, config.SeedSelector == nil, seedName)
	if err != nil {
		return err
	}

	dnsClient, err := dnsclientset.NewForConfig(k8sSeedClient.RESTConfig())
	if err != nil {
		return err
	}
	extensionsClient, err := extensionsclientset.NewForConfig(k8sSeedClient.RESTConfig())
	if err != nil {
		return err
	}

	var (
		childCtx, cancelFunc = context.WithCancel(ctx)
		seedLogger           = logger.Logger.WithField("Seed", seedName)

		dnsInformers         = dnsinformers.NewSharedInformerFactory(dnsClient, 0)
		extensionsInformers  = extensionsinformers.NewSharedInformerFactory(extensionsClient, 0)
		extensionsController = extensions.NewController(ctx, k8sGardenClient, k8sSeedClient, seedName, dnsInformers, extensionsInformers, seedLogger, recorder)
	)

	logger.Logger.Infof("Run extensions controller for seed: %s", seedName)
	if err := extensionsController.Run(childCtx, *config.Controllers.ControllerInstallationRequired.ConcurrentSyncs, *config.Controllers.ShootStateSync.ConcurrentSyncs); err != nil {
		logger.Logger.Infof("There was error running the extensions controller for seed: %s", seedName)
		cancelFunc()
		return err
	}

	f.addController(seedName, extensionsController, cancelFunc)
	return nil
}

func (f *federatedSeedControllerManager) addController(seedName string, controller *extensions.Controller, cancelFunc context.CancelFunc) {
	logger.Logger.Infof("Adding controllers for seed %s", seedName)
	f.lock.Lock()
	defer f.lock.Unlock()
	f.controllers[seedName] = &extensionsController{
		controller: controller,
		cancelFunc: cancelFunc,
	}
}

func (f *federatedSeedControllerManager) removeController(seedName string) {
	if controller, ok := f.controllers[seedName]; ok {
		logger.Logger.Infof("Removing controllers for seed %s", seedName)
		controller.Stop()
		f.lock.Lock()
		defer f.lock.Unlock()
		delete(f.controllers, seedName)
	}
}

func (f *federatedSeedControllerManager) ShutDownControllers() {
	logger.Logger.Infof("Federated controllers are being stopped")
	for _, controller := range f.controllers {
		controller.Stop()
	}
}
